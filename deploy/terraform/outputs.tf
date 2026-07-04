output "public_ip" {
  description = "Elastic IP of the instance"
  value       = aws_eip.app.public_ip
}

output "ssh_command" {
  description = "SSH into the box"
  value       = "ssh ec2-user@${aws_eip.app.public_ip}"
}

output "gateway_url" {
  description = "Point the Godot client's ApiClient.BASE_URL here"
  value       = "http://${aws_eip.app.public_ip}:8088/v1"
}

output "battle_ws_url" {
  description = "Battle WebSocket base (client connects here for PvP)"
  value       = "ws://${aws_eip.app.public_ip}:8082"
}

output "first_boot_note" {
  value = "First boot builds Docker images on the instance (~10-20 min on t3.micro). Watch progress with: sudo tail -f /var/log/beastbound-deploy.log"
}
