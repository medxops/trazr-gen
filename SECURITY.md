# Security Policy

Thank you for helping to keep this project secure!

## Reporting a Vulnerability

Please report security issues to [your security contact email] or via GitHub Security Advisories. We will respond as quickly as possible to address the issue.

---

## CI/CD and Repository Security Best Practices

This project follows strict security and privacy practices, suitable for healthcare and regulated environments:

### Branch Protection and Required Status Checks
- Branch protection rules are enabled for `main`.
- All status checks (build, test, lint, Snyk, code scanning) must pass before merging.

### Dependabot Alerts and Security Updates
- Dependabot alerts and security updates are enabled to monitor for vulnerable dependencies.

### Secret Scanning and Push Protection
- Secret scanning and push protection are enabled to prevent accidental exposure of secrets.

### Workflow Permissions
- Default workflow permissions are set to read-only.
- Jobs that require write access (e.g., releases) explicitly request it.

### Secrets Management
- All sensitive secrets (GPG, Snyk, Docker, etc.) are stored in GitHub Secrets and rotated regularly.

### GPG Key Handling
- GPG keys are imported, trusted, and cleaned up securely in CI/CD workflows.

### OIDC for Cloud Deployments
- OIDC is used for cloud deployments where possible, avoiding static credentials.

### SBOM and Supply Chain Security
- SBOMs are generated for every release and uploaded to GitHub as both artifacts and via the SBOM API.

### Snyk and Code Scanning
- Snyk scans are run on every PR and push to main.
- SARIF results are uploaded to GitHub Code Scanning for visibility in the Security tab.

### Audit Logging
- Audit logging is enabled at the organization level for compliance and incident response.

### Third-Party Actions
- Only trusted, pinned actions are used in workflows.

### Documentation
- This policy and CI/CD security controls are documented and reviewed regularly.

---

For any questions or to report a security concern, please contact the maintainers.

## Verifying Releases

You can verify signed releases and checksums using our [public GPG key](https://github.com/medxops/trazr-gen/public.key).

## Supported Versions

We generally support the latest major and minor releases. Please check the [releases page](https://github.com/medxops/trazr-gen/changelog.md) for details.
