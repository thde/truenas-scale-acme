# truenas-scale-acme

[![Go Report Card](https://goreportcard.com/badge/github.com/thde/truenas-scale-acme)](https://goreportcard.com/report/github.com/thde/truenas-scale-acme)

`truenas-scale-acme` optains and manages certificates for TrueNAS Scale using the [ACME DNS-01 challenge](https://letsencrypt.org/docs/challenge-types/#dns-01-challenge) and the [TrueNAS Scale API](https://www.truenas.com/docs/api/scale_rest_api.html).

It uses Caddy's [caddyserver/certmagic](https://github.com/caddyserver/certmagic) library internally to optain and renew SSL certificates and ensures that TrueNAS uses a valid certificate to serve requests.

## Supported DNS-Providers

Currently the following providers are supported:

- [acme-dns](https://github.com/joohoi/acme-dns)
- [cloudflare](https://github.com/libdns/cloudflare)

If you require a different provider, feel free to create an issue. In theory, all [github.com/libdns](https://github.com/orgs/libdns/repositories?q=&type=all&language=&sort=stargazers) providers can be supported.

## Install

### Homebrew

```shell
brew tap thde/truenas-scale-acme
brew install thde/truenas-scale-acme/truenas-scale-acme
```

### curl

```shell
mkdir truenas-scale-acme
curl -L $(curl -s https://api.github.com/repos/thde/truenas-scale-acme/releases/latest |
    jq -r '.assets[].browser_download_url | select(contains ("linux_amd64"))') |
    tar xvz -C ./truenas-scale-acme
```

## Getting Started

1. [Create](https://www.truenas.com/docs/scale/scaletutorials/toptoolbar/managingapikeys/) an API key in TrueNAS
1. Register an account on ACME-DNS server:
   ```shell
   curl -X POST https://auth.acme-dns.io/register
   ```
1. Create a DNS CNAME record that points from `_acme-challenge.your-domain.example.com` to the `fulldomain` from the registration response.
1. Use the credentials obtained in step 1 and 2 to configure truenas-scale-acme (default `~/.config/truenas-scale-acme/config.json`):
   ```json
   {
     "domain": "nas.domain.com",
     "scale": {
       "api_key": "s3cure",
       "url": "https://localhost/api/v2.0/",
       "skip_verify": true
     },
     "acme": {
       "email": "myemail@example.com",
       "tos_agreed": true,
       "acme-dns": {
         "username": "00000000-0000-0000-0000-000000000000",
         "password": "s3cure",
         "subdomain": "FFFFFFFF-FFFF-FFFF-FFFF-FFFFFFFFFFFF",
         "server_url": "https://auth.acme-dns.io"
       }
     }
   }
   ```
1. Run `truenas-acme-scale` and verify that the certificate is issued and updated successfully.
1. Setup a cronjob that runs `truenas-acme-scale` daily as the correct user.

## CA's

`truenas-scale-acme` currently has the following CA's configured by default:

1. Let's Encrypt
2. ZeroSSL

This ensures a valid certificate even if one CA is unavailable.

## Other Solutions

- [TrueNAS SCALE/ACME Certificates](https://www.truenas.com/docs/scale/scaletutorials/credentials/certificates/settingupletsencryptcertificates/) - TrueNAS Scale integrated ACME functionality using DNS authentication. Includes support for external [shell commands](https://www.truenas.com/community/threads/howto-acme-dns-authenticator-shell-script-using-acmesh-project.107252/).
- [danb35/deploy-freenas](https://github.com/danb35/deploy-freenas) - Python script to deploy TLS certificates to a TrueNAS Core using its API.
- [acmesh-official/acme.sh/deploy/truenas.sh](https://github.com/acmesh-official/acme.sh/wiki/deployhooks#25-deploy-the-cert-on-truenas-core-server) - acme.sh deploy script for TrueNAS Core using its API.
