# tf-man

a basic tf workflow is:

```hcl
terraform init # initialize tf to working directory and download providers/modules
terraform fmt # format acc to tf standard formatting rules
terraform validate # checks for tf config syntactically and structurally valid
terraform plan # shows what changes will tf make to infra
terraform apply # apply those changes
terraform destroy # delete the infra
```
