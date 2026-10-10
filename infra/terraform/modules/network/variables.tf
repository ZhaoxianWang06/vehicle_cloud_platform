variable "name" { type = string }
variable "cidr" { type = string }
variable "cluster_name" { type = string }
variable "nat_per_az" { type = bool }
variable "tags" { type = map(string) }
