resource "aws_instance" "web" {
  for_each = aws_subnet.public

  ami                    = data.aws_ami.amazon_linux.id
  instance_type          = var.instance_type
  iam_instance_profile   = aws_iam_instance_profile.ssm_profile.name
  subnet_id              = each.value.id
  vpc_security_group_ids = [aws_security_group.instance_sg.id]

  user_data = templatefile("${path.module}/user-data/bootstrap.sh.tftpl", {
    instance_name = "${var.instance_name}-${each.key}"
  })

  tags = {
    Name = "${var.instance_name}-${each.key}"
  }
}
