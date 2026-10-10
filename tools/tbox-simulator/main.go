package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// 用多辆车并发上报，模拟 T-Box 车队。这是观察 Nginx 限流和上报延迟的入口。
func main() {
	base := flag.String("url", "http://127.0.0.1:18080", "Nginx 地址")
	vehicles := flag.Int("vehicles", 10, "模拟车辆数")
	interval := flag.Duration("interval", 200*time.Millisecond, "每辆车的上报间隔")
	duration := flag.Duration("duration", 10*time.Second, "压测时长")
	withAck := flag.Bool("ack", true, "车控下发后立即模拟车端回执")
	flag.Parse()

	client := &http.Client{Timeout: 3 * time.Second}
	var okCount, errCount atomic.Int64
	var mu sync.Mutex
	latencies := make([]time.Duration, 0, (*vehicles)*int(duration.Seconds()/interval.Seconds())+8)

	var wg sync.WaitGroup
	deadline := time.Now().Add(*duration)
	for i := 0; i < *vehicles; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			vin := fmt.Sprintf("LSVAA4182E%07d", n)
			seq := 0
			for time.Now().Before(deadline) {
				seq++
				body := map[string]any{
					"vin": vin, "speed_kmh": float64(40 + n%30), "battery_pct": float64(50 + n%40),
					"lng": 121.4 + float64(n)/1000, "lat": 31.2, "odometer_km": 1000 + float64(seq),
					"reported_at": time.Now().UTC().Format(time.RFC3339),
				}
				status, latency := postJSON(client, *base+"/api/v1/telemetry", body)
				record(&mu, &latencies, &okCount, &errCount, status, latency)
				if seq%25 == 0 {
					key := fmt.Sprintf("sim-%s-%d", vin, seq)
					status, latency = postJSON(client, *base+"/api/v1/commands", map[string]any{
						"vin": vin, "command": "horn", "idempotency_key": key,
					})
					record(&mu, &latencies, &okCount, &errCount, status, latency)
					if *withAck && status == http.StatusAccepted {
						// 回执失败只计入错误，不中断这辆车的上报。
						var created struct {
							Data struct {
								ID string `json:"id"`
							} `json:"data"`
						}
						// 简化：回执需要 id。上面的 postJSON 没返回 body，这里再发一次相同幂等键拿到 id。
						raw := postRaw(client, *base+"/api/v1/commands", map[string]any{
							"vin": vin, "command": "horn", "idempotency_key": key,
						})
						_ = json.Unmarshal(raw.body, &created)
						if created.Data.ID != "" {
							st, lat := postJSON(client, *base+"/api/v1/commands/"+created.Data.ID+"/ack", map[string]any{
								"success": true, "message": "horn acked",
							})
							record(&mu, &latencies, &okCount, &errCount, st, lat)
						}
					}
				}
				time.Sleep(*interval)
			}
		}(i)
	}
	wg.Wait()

	mu.Lock()
	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
	p99 := time.Duration(0)
	if n := len(latencies); n > 0 {
		p99 = latencies[(n*99)/100]
	}
	mu.Unlock()

	total := okCount.Load() + errCount.Load()
	qps := float64(total) / duration.Seconds()
	fmt.Printf("vehicles=%d duration=%s ok=%d err=%d qps=%.1f p99=%s\n",
		*vehicles, duration.String(), okCount.Load(), errCount.Load(), qps, p99)
	if errCount.Load() > 0 && float64(errCount.Load())/float64(max(total, 1)) > 0.01 {
		os.Exit(1)
	}
}

type rawResult struct {
	status int
	body   []byte
}

func postRaw(client *http.Client, url string, payload any) rawResult {
	buf, _ := json.Marshal(payload)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(buf))
	if err != nil {
		return rawResult{status: 599}
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return rawResult{status: 599}
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	return rawResult{status: resp.StatusCode, body: body}
}

func postJSON(client *http.Client, url string, payload any) (int, time.Duration) {
	start := time.Now()
	res := postRaw(client, url, payload)
	return res.status, time.Since(start)
}

func record(mu *sync.Mutex, latencies *[]time.Duration, okCount, errCount *atomic.Int64, status int, latency time.Duration) {
	mu.Lock()
	*latencies = append(*latencies, latency)
	mu.Unlock()
	if status >= 200 && status < 300 {
		okCount.Add(1)
		return
	}
	errCount.Add(1)
}
