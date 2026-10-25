# Terraform review: Lambda + API Gateway + S3

Scope: `terraform/versions.tf`, `provider.tf`, `variables.tf`, `main.tf`, and
`outputs.tf`. The configuration targets Floci locally; the default provider
credentials are deliberately fake and must not be reused for AWS.

## Line-by-line review

| File / lines | What each line or block does | Risk | Changed? Why? |
|---|---|---|---|
| `versions.tf` 1-2 | Starts Terraform settings and requires Terraform 1.5 or newer. | A newer local version may change provider behavior; version floor is not an upper bound. | No. The feature set needs a modern Terraform release. |
| `versions.tf` 4-8 | Declares the AWS provider source and accepts compatible `6.x` versions. | Provider changes inside a major release can still affect emulator compatibility. | No. This follows the assignment prompt; pin a lockfile for repeatable team builds. |
| `provider.tf` 1-2 | Documents that all service APIs are routed to Floci. | A mistaken endpoint can send provisioning to a real account or a different emulator. | No. Keep the endpoint configurable and verify plans before apply. |
| `provider.tf` 3-4 | Configures the AWS provider and selects the region variable. | Region is a deployment boundary and affects resource ARNs. | No. Region is variable-driven. |
| `provider.tf` 5-6 | Supplies Floci's dummy access key and secret. | These are not credentials; copying this provider to AWS would fail and is unsafe practice. | No. Required only by the local emulator. |
| `provider.tf` 8-11 | Disables AWS identity/metadata checks and selects S3 path-style URLs. | These bypass checks and are appropriate only for the emulator. | No. Required for Floci compatibility. |
| `provider.tf` 13-20 | Routes API Gateway, CloudWatch Logs, IAM, Lambda, S3, and STS calls to the endpoint variable. | Missing an endpoint may cause Terraform to contact AWS; endpoint aliases vary by provider version. | No. Each service used by this stack is routed locally. |
| `variables.tf` 1-5 | Defines the region as a string with `us-east-1` default. | A default can hide an unintended deployment region. | No. It is a conventional Floci region and remains overridable. |
| `variables.tf` 7-11 | Defines the Floci endpoint with the local URL default. | Localhost only works when Terraform and Floci run on the same host/network namespace. | No. It is the expected local setup. |
| `variables.tf` 13-27 | Defines the Lambda architecture, default, description, and validation allow-list. | A mismatch between `GOARCH`, image host, and Lambda architecture causes invocation failures. | No. `scripts/env.sh` derives the architecture for local builds. |
| `variables.tf` 29-37 | Names the input and output S3 buckets. | Fixed bucket names can collide in shared AWS accounts; Floci is local and isolated. | No. Names are configurable, and the demo expects stable local names. |
| `variables.tf` 39-42 | Defines the API stage, defaulting to `dev`. | A public production stage would need authentication and throttling. | No. This is a local exercise endpoint. |
| `main.tf` 1-10 | Creates the covers and thumbnails buckets using the bucket-name variables. `force_destroy = true` allows Terraform to delete non-empty buckets. | Destructive: applying destroy can permanently remove objects. | Kept for disposable local demo buckets; remove or set false for real data. |
| `main.tf` 12-21 | Builds the Lambda trust policy: only the Lambda service may assume this role. | Trust policy is security-sensitive; adding principals would grant role assumption. | No. It is scoped to `lambda.amazonaws.com`. |
| `main.tf` 23-26 | Creates the execution role and attaches the trust document. | The role by itself grants no service permissions. | No. Permissions are granted separately below. |
| `main.tf` 28-46 | Creates least-privilege statements: log writes only to the two named Lambda log groups, `GetObject` from covers, and `PutObject` to thumbnails. | Region/account remain wildcarded because they are emulator/account-specific; log groups are limited to the two functions. | **Yes.** Replaced the broad `libratrack-*` log prefix with the exact webhook and thumbnail groups. The S3 ARNs remain limited to the two dedicated buckets; no `Action = "*"`. |
| `main.tf` 48-52 | Attaches the generated permissions policy to the execution role. | Policy edits change runtime authority; review before applying. | No. This keeps permissions auditable as a separate policy resource. |
| `main.tf` 53-67 | Creates the HTTP webhook Lambda: role, `provided.al2023`, matching architecture, `bootstrap`, ZIP path/hash, 10-second timeout, and 256 MB memory. | `function_name` is fixed; timeout/memory are initial estimates; ZIP must exist before plan. | No. Runtime is supported and architecture comes from a validated variable. Re-measure timeout and memory before production. |
| `main.tf` 69-88 | Creates the thumbnail Lambda with the same runtime/package conventions, a longer timeout, 512 MB, and the output bucket environment variable. | Fixed function name; memory/timeout should be confirmed by measurements. | No. Image processing needs more CPU/memory; output bucket is injected from the created resource. |
| `main.tf` 90-93 | Creates a REST API Gateway API. | The API does not define authentication by itself. | No. This is a minimal local webhook demo. |
| `main.tf` 95-99 | Creates the `/webhook` resource beneath the API root. | A single route is intentionally narrow. | No. It is the HTTP trigger required by the assignment. |
| `main.tf` 101-106 | Allows `POST` and sets API Gateway authorization to `NONE`. | Anyone who can reach the endpoint can invoke it; no auth or throttling is configured. | No. Local exercise only. Add auth, throttling, and validation for a public deployment. |
| `main.tf` 108-115 | Wires the POST method to Lambda using the AWS proxy integration and Lambda invoke ARN. | Proxy event/response format is a contract the handler must satisfy. | No. This preserves client status codes such as 400 rather than turning validation errors into 502s. |
| `main.tf` 120-125 | Grants API Gateway permission to invoke the webhook Lambda. | Stage name is variable-driven; an incorrect stage variable would prevent invocation. | **Yes.** Replaced `*/*` with this API's configured stage, `POST`, and `/webhook`. This follows API Gateway's execute-api ARN format and limits the invoking route. Confirm the emulator's enforcement with Terraform plan/apply before relying on it. |
| `main.tf` 125-139 | Creates a deployment, hashes route/method/integration IDs to trigger redeployment, and creates the new deployment before destroying the old one. | Hashing IDs may not notice every future API configuration change if more settings are added. | No. This prevents stale stage deployments after route wiring changes and reduces downtime. |
| `main.tf` 141-145 | Creates the named API stage linked to the deployment. | Stage is reachable according to the API authorization configuration. | No. Stage name is configurable and defaults to `dev`. |
| `main.tf` 147-154 | Grants S3 permission to invoke the thumbnail Lambda, restricted to the covers bucket ARN. | Bucket-level source is appropriate here; adding buckets broadens the trigger trust. | No. The source ARN is scoped to the exact input bucket. |
| `main.tf` 156-165 | Configures object-created notifications to invoke the thumbnail function and waits for invoke permission first. | All created objects can trigger processing; avoid writing outputs back into the input bucket to prevent loops. | No. Input and output buckets are separate, avoiding recursive notification loops. |
| `outputs.tf` 1-3 | Exposes API Gateway's generated ID. | ID is not secret, but outputs are persisted in Terraform state. | No. Useful for smoke tests. |
| `outputs.tf` 5-8 | Builds a direct Floci URL using endpoint, API ID, stage, and `_user_request_`. | This is emulator-specific and not a real AWS URL. | No. The output name explicitly identifies Floci. |
| `outputs.tf` 10-12 | Prints a ready-to-run curl command using the valid payload fixture. | Shell quoting may need adjustment if endpoint/path contains special characters. | No. It shortens the local test loop. |

## Plan observation

Ran `make plan-tf` against local Floci after `terraform init` and
`terraform validate` succeeded. Terraform 1.17.0 selected AWS provider 6.70.0.
The plan reports **15 to add, 0 to change, 0 to destroy**; `apply` was not run.
Terraform also reads the IAM policy document as a data source and shows the API
ID, webhook URL, and curl command as outputs.

The unexpected part was that planning the HTTP function also creates the
thumbnail Lambda, both S3 buckets, S3 invoke permission, and bucket notification.
These are intentional because Terraform describes the entire lesson demo stack,
not only the webhook. New IDs and ARNs appear as `(known after apply)`, which is
expected for resources not yet created. The resources were already created in
Floci through the AWS CLI path, which has no Terraform state; the plan therefore
does not know about them. Applying this plan to the same emulator could cause
name conflicts, so `apply` was deliberately not run. To apply safely, first use
a clean Floci state or import the existing resources into Terraform state.

## Summary

The most consequential settings are `force_destroy = true` and unauthenticated
`POST /webhook`. They are tolerable for a disposable local emulator exercise,
but they must be revisited before using the configuration in a real AWS
account. The Lambda role avoids broad wildcard actions, scopes log writes to
the two functions, and limits S3 object access to the demo buckets.