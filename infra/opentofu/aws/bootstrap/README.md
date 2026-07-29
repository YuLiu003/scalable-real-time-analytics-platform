# AWS state foundation

This independently owned stack creates the encrypted, versioned S3 backend used
by the AWS lab. It deliberately uses local state for its first apply because a
remote backend cannot create itself. Store that bootstrap state securely.

The state bucket has `prevent_destroy`; deleting it is a separate, deliberate
administrative procedure after all dependent state has been migrated or
destroyed.

```bash
cp terraform.tfvars.example terraform.tfvars
tofu init
tofu plan -out bootstrap.tfplan
tofu apply bootstrap.tfplan
```

Do not pass credentials in backend arguments or commit `terraform.tfvars`, plan
files, state, or `.terraform/`.
