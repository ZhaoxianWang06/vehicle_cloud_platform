output "endpoint" { value = aws_db_instance.this.address }
output "password" {
  value     = random_password.this.result
  sensitive = true
}
