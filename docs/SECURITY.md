# Security Guide

This document outlines security best practices for deploying and operating DBEaseBackup.

## Table of Contents

- [Credential Management](#credential-management)
- [Database Access](#database-access)
- [Service Account Security](#service-account-security)
- [S3 Security](#s3-security)
- [Network Security](#network-security)
- [Backup Retention](#backup-retention)
- [Security Checklist](#security-checklist)

## Credential Management

### Never Commit Secrets

**Never commit to version control:**
- `.env` files
- `service-account-key.json` (Google Drive)
- Any file containing passwords or API keys

Add to `.gitignore`:
```
.env
service-account-key.json
*.pem
*.key
```

### Use Environment Variables

Pass sensitive configuration through environment variables:

```bash
# Docker run
docker run -e PG_PASSWORD=secret -e S3_SECRET_ACCESS_KEY=secret ...

# Docker Compose with .env
# .env is not committed
env_file:
  - .env

# Kubernetes Secrets
env:
  - name: PG_PASSWORD
    valueFrom:
      secretKeyRef:
        name: dbeasebackup-secrets
        key: pg-password
```

### Use Secrets Management

For production deployments, use a secrets management solution:

**Docker Secrets:**
```yaml
version: '3.8'
services:
  dbeasebackup:
    image: glennprays/dbeasebackup:latest
    secrets:
      - pg_password
      - s3_secret_key
secrets:
  pg_password:
    external: true
  s3_secret_key:
    external: true
```

**Kubernetes Secrets:**
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: dbeasebackup-secrets
type: Opaque
stringData:
  pg-password: your-secure-password
  s3-secret-key: your-secret-key
```

**HashiCorp Vault:**
```bash
# Retrieve secrets from Vault
export PG_PASSWORD=$(vault kv get -field=password secret/dbeasebackup/postgres)
```

### Rotate Credentials Regularly

- Rotate database passwords every 90 days
- Rotate S3 access keys every 90 days
- Rotate service account keys annually or when compromised
- Update application configuration after rotation

## Database Access

### Create a Dedicated Backup User

Use a dedicated database user with minimal privileges:

```sql
-- Create backup user
CREATE USER backup_user WITH PASSWORD 'secure_random_password';

-- Grant connection permission
GRANT CONNECT ON DATABASE your_database TO backup_user;

-- Grant schema usage
GRANT USAGE ON SCHEMA public TO backup_user;

-- Grant read access to all tables
GRANT SELECT ON ALL TABLES IN SCHEMA public TO backup_user;

-- Grant read access to future tables (PostgreSQL 9.0+)
ALTER DEFAULT PRIVILEGES IN SCHEMA public
  GRANT SELECT ON TABLES TO backup_user;

-- For pg_dump to read sequences
GRANT SELECT ON ALL SEQUENCES IN SCHEMA public TO backup_user;
```

### Principle of Least Privilege

The backup user should only have:
- `CONNECT` - Connect to the database
- `USAGE` - Use schemas
- `SELECT` - Read table data
- No INSERT, UPDATE, DELETE, or DDL privileges

### Secure Password Generation

Generate strong passwords:

```bash
# OpenSSL (32 characters)
openssl rand -base64 32

# Python
python3 -c "import secrets; print(secrets.token_urlsafe(32))"

# 1Password, Bitwarden, or other password managers
```

### Restrict Connection Sources

In `pg_hba.conf`:

```
# Only allow connections from backup server
host    your_database    backup_user    192.168.1.100/32    scram-sha-256
```

## Service Account Security

### Google Service Account Best Practices

1. **Use a dedicated service account**
   - Create a service account specifically for backups
   - Don't reuse service accounts across applications

2. **Limit scope**
   - Only grant access to specific Google Drive folders
   - Don't grant organization-wide permissions

3. **Key management**
   - Store the JSON key securely
   - Use key rotation (automated if possible)
   - Delete old keys after rotation

4. **Audit access**
   - Review service account activity in Google Cloud Console
   - Set up audit logs for suspicious activity

### Service Account Key Storage

```bash
# Secure file permissions
chmod 600 service-account-key.json

# Docker volume with restricted access
docker run -v $(pwd)/service-account-key.json:/service-account-key.json:ro ...

# Kubernetes Secret
kubectl create secret generic google-key --from-file=key.json=service-account-key.json
```

## S3 Security

### IAM Best Practices

1. **Use IAM roles when possible** (EC2, ECS, EKS)
   ```json
   {
     "Version": "2012-10-17",
     "Statement": [
       {
         "Effect": "Allow",
         "Action": [
           "s3:PutObject",
           "s3:GetObject",
           "s3:ListBucket",
           "s3:DeleteObject"
         ],
         "Resource": [
           "arn:aws:s3:::your-backup-bucket",
           "arn:aws:s3:::your-backup-bucket/*"
         ]
       }
     ]
   }
   ```

2. **Use dedicated IAM user for backups**
   - Create a user with only S3 access
   - Don't use root credentials
   - Don't share credentials across applications

3. **Enable MFA for IAM users**
   - Require MFA for console access
   - Consider MFA for API access in high-security environments

### S3 Bucket Security

1. **Enable encryption at rest**
   ```bash
   aws s3api put-bucket-encryption \
     --bucket your-backup-bucket \
     --server-side-encryption-configuration '{
       "Rules": [{
         "ApplyServerSideEncryptionByDefault": {
           "SSEAlgorithm": "AES256"
         }
       }]
     }'
   ```

2. **Enable versioning**
   ```bash
   aws s3api put-bucket-versioning \
     --bucket your-backup-bucket \
     --versioning-configuration Status=Enabled
   ```

3. **Block public access**
   ```bash
   aws s3api put-public-access-block \
     --bucket your-backup-bucket \
     --public-access-block-configuration \
     "BlockPublicAcls=true,IgnorePublicAcls=true,BlockPublicPolicy=true,RestrictPublicBuckets=true"
   ```

4. **Enable access logging**
   ```bash
   aws s3api put-bucket-logging \
     --bucket your-backup-bucket \
     --bucket-logging-status '{
       "LoggingEnabled": {
         "TargetBucket": "log-bucket",
         "TargetPrefix": "s3-access-logs/"
       }
     }'
   ```

### S3-Compatible Storage

For MinIO, DigitalOcean Spaces, etc.:

1. **Use HTTPS endpoints** (set `S3_ENDPOINT` with https://)
2. **Create dedicated access keys**
3. **Enable bucket policies** for access control
4. **Use TLS** for data in transit

## Network Security

### Docker Network Configuration

1. **Avoid `--network host` in production**
   ```yaml
   # Instead, use Docker networks
   networks:
     - backend

   networks:
     backend:
       driver: bridge
   ```

2. **Limit container capabilities**
   ```yaml
   cap_drop:
     - ALL
   cap_add:
     - NET_BIND_SERVICE
   read_only: true
   ```

### TLS/SSL for PostgreSQL

Enable SSL for database connections in production:

```bash
# In postgresql.conf
ssl = on
ssl_cert_file = '/path/to/server.crt'
ssl_key_file = '/path/to/server.key'

# Connection string
PG_HOST=your-db.example.com
# SSL is negotiated automatically if configured on server
```

### Firewall Rules

Restrict inbound/outbound traffic:

```
# Allow only necessary outbound
- Database port (5432)
- Google Drive API (443)
- S3 API (443)

# Deny all inbound (no services exposed)
```

## Backup Retention

### Retention Policy

Define a clear retention policy:

| Backup Type | Retention | Storage Location |
|-------------|-----------|------------------|
| Hourly | 24 hours | Primary |
| Daily | 30 days | Primary + Archive |
| Weekly | 12 weeks | Archive |
| Monthly | 12 months | Archive + Cold Storage |

### Encryption of Backups

1. **At rest** (S3, Google Drive)
   - Use bucket-level encryption (S3)
   - Google Drive encrypts by default

2. **In transit**
   - TLS for all uploads (automatic)

3. **Client-side encryption** (optional, for sensitive data)
   - Encrypt before upload
   - Manage encryption keys separately

### Secure Deletion

When deleting old backups:
- S3: Use lifecycle policies or versioning with expiration
- Google Drive: Empty trash after deletion

## Security Checklist

### Before Deployment

- [ ] All secrets stored securely (not in code)
- [ ] Dedicated backup database user created
- [ ] Minimal privileges granted
- [ ] Service account has minimal permissions
- [ ] S3 bucket is private with encryption enabled
- [ ] Network access is restricted
- [ ] TLS enabled for all connections

### Regular Maintenance

- [ ] Rotate database passwords (every 90 days)
- [ ] Rotate S3 access keys (every 90 days)
- [ ] Rotate service account keys (annually)
- [ ] Review access logs
- [ ] Audit IAM permissions
- [ ] Update to latest version

### Incident Response

If credentials are compromised:

1. **Immediately rotate** the affected credentials
2. **Review access logs** for unauthorized access
3. **Check backup integrity** - ensure backups weren't tampered with
4. **Update application** with new credentials
5. **Document the incident** for future reference

## Reporting Security Issues

If you discover a security vulnerability in DBEaseBackup:

1. **Do not** open a public issue
2. Email security concerns to the maintainers
3. Include:
   - Description of the vulnerability
   - Steps to reproduce
   - Potential impact
   - Suggested fix (if any)

We take security seriously and will respond promptly to reported issues.
