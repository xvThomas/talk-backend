# MCP Server Authentication

The MCP server supports two authentication mechanisms for the HTTP transport.
They can be used independently or combined (the server accepts either one).

## Table of Contents

- [MCP Server Authentication](#mcp-server-authentication)
  - [Table of Contents](#table-of-contents)
  - [1. API Key Authentication](#1-api-key-authentication)
    - [Environment Variables](#environment-variables)
    - [Flow](#flow)
  - [2. OAuth 2.0 Authentication](#2-oauth-20-authentication)
    - [2.1. Concepts](#21-concepts)
    - [2.2. Direct Mode (without proxy)](#22-direct-mode-without-proxy)
    - [2.3. Proxy Mode (for Claude.ai / Auth0)](#23-proxy-mode-for-claudeai--auth0)
  - [3. Auth0 Setup Step by Step](#3-auth0-setup-step-by-step)
    - [3.1. Create the API (Resource Server)](#31-create-the-api-resource-server)
    - [3.2. Create the Application (OAuth Client)](#32-create-the-application-oauth-client)
    - [3.3. Authorize the Application to Access the API](#33-authorize-the-application-to-access-the-api)
  - [4. Keycloak Setup Step by Step](#4-keycloak-setup-step-by-step)
    - [4.1. Create a Realm](#41-create-a-realm)
    - [4.2. Create a Client (Confidential)](#42-create-a-client-confidential)
    - [4.3. Add an Audience Mapper](#43-add-an-audience-mapper)
    - [4.4. (Optional) Create a User for Testing](#44-optional-create-a-user-for-testing)
  - [5. `.env` Configuration](#5-env-configuration)
    - [5.1. API Key Only](#51-api-key-only)
    - [5.2. OAuth without Proxy](#52-oauth-without-proxy)
    - [5.3. OAuth with Proxy (Auth0 + Claude)](#53-oauth-with-proxy-auth0--claude)
    - [5.4. OAuth with Proxy (Keycloak + Claude)](#54-oauth-with-proxy-keycloak--claude)
    - [5.5. API Key + OAuth Combined](#55-api-key--oauth-combined)
  - [6. Client Configuration](#6-client-configuration)
    - [6.1. VS Code / Copilot (API Key)](#61-vs-code--copilot-api-key)
    - [6.2. Claude.ai (OAuth via Proxy)](#62-claudeai-oauth-via-proxy)
  - [7. Troubleshooting](#7-troubleshooting)
    - [Common Errors](#common-errors)

---

## 1. API Key Authentication

The client sends a shared secret in the `X-API-Key` header with every request.

**Pros**: simple, no external dependency.
**Cons**: the secret is static, no user identity.

### Environment Variables

| Variable    | Description                             | Required |
| ----------- | --------------------------------------- | -------- |
| `X_API_KEY` | Shared secret between client and server | Yes      |

### Flow

```
Client                          MCP Server
  │                                  │
  │── POST /mcp ────────────────────>│
  │   Header: X-API-Key: <secret>    │
  │                                  │── Verify X-API-Key
  │<──────────── 200 OK ─────-───────│
```

---

## 2. OAuth 2.0 Authentication

The MCP server acts as a **Resource Server** (RFC 9728). An external **Authorization Server** (Auth0, Keycloak…) issues JWT access tokens that the server validates via JWKS.

### 2.1. Concepts

| Role                     | Description                                                                                                                                                          |
| ------------------------ | -------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **OAuth Client**         | Claude.ai, Copilot, or any MCP app                                                                                                                                   |
| **Authorization Server** | Auth0, Keycloak… — issues tokens                                                                                                                                     |
| **Resource Server**      | The MCP server — validates tokens and serves tools                                                                                                                   |
| **AS Proxy**             | Local endpoints on the MCP server that intercept `/authorize` and `/token` to inject required parameters (audience, offline_access) before forwarding to the real AS |

### 2.2. Direct Mode (without proxy)

The OAuth client talks directly to the Authorization Server. The MCP server only validates the received JWT.

This mode works when the client correctly sends the `audience` parameter in the authorization request (which Auth0 requires to issue a JWT).

```
Client                    Authorization Server          MCP Server
  │                              │                          │
  │── GET /authorize ───────────>│                          │
  │   (with audience=owm-mcp)    │                          │
  │<──── redirect + code ─────-──│                          │
  │── POST /token ──────────────>│                          │
  │<──── access_token (JWT) ──-──│                          │
  │                              │                          │
  │── POST /mcp ───────────────────────────────────────────>│
  │   Header: Authorization: Bearer <JWT>                   │
  │                              │                          │── Validate JWT (JWKS)
  │<──────────────────────────── 200 OK ────────────────-───│
```

### 2.3. Proxy Mode (for Claude.ai / Auth0)

Some OAuth clients (such as Claude.ai) **do not send** the `audience` parameter in the authorization request. However, Auth0 requires it to issue a JWT (otherwise it returns an unusable opaque token).

The AS proxy solves this problem by exposing three local endpoints:

| Endpoint                                  | Role                                                        |
| ----------------------------------------- | ----------------------------------------------------------- |
| `/.well-known/oauth-authorization-server` | AS Metadata (RFC 8414) pointing to the proxy endpoints      |
| `/authorize`                              | Injects `audience` and `offline_access`, redirects to Auth0 |
| `/token`                                  | Forwards to Auth0, injects `client_secret` if configured    |

```txt
Client              MCP Server (proxy)           Auth0
  │                        │                        │
  │── POST /mcp ──────────>│                        │
  │<── 401 + WWW-Auth ───-─│                        │
  │                        │                        │
  │── GET /.well-known/ ──>│                        │
  │   oauth-protected-     │                        │
  │   resource             │                        │
  │<── {authorization_ ─-──│                        │
  │     servers: [self]}   │                        │
  │                        │                        │
  │── GET /.well-known/ ──>│                        │
  │   oauth-authorization- │                        │
  │   server               │                        │
  │<── {endpoints proxy} -─│                        │
  │                        │                        │
  │── GET /authorize ─────>│                        │
  │                        │── + audience=owm-mcp ─>│
  │                        │── + offline_access ───>│
  │<── redirect Auth0 ───-─│<── redirect + code ─-──│
  │                        │                        │
  │── POST /token ────────>│                        │
  │                        │── + client_secret ────>│
  │<── access_token ────-──│<── JWT + refresh ───-──│
  │                        │                        │
  │── POST /mcp ──────────>│                        │
  │   Bearer <JWT>         │── Validate JWT (JWKS)  │
  │<── 200 OK ───────────-─│                        │
```

---

## 3. Auth0 Setup Step by Step

### 3.1. Create the API (Resource Server)

The API represents your MCP server in Auth0. It is the `audience` of the tokens.

1. Log in to the [Auth0 Dashboard](https://manage.auth0.com)
2. Sidebar → **Applications** → **APIs**
3. Click **+ Create API**
4. Fill in:
   - **Name**: `OWM MCP` (or your server name)
   - **Identifier**: `owm-mcp` (this is the `audience` value — it cannot be changed after creation)
   - **Signing Algorithm**: `RS256`
5. Click **Create**
6. In the **Settings** of the created API:
   - **Allow Offline Access**: **enabled** (toggle ON) — required for Auth0 to return a `refresh_token`
7. Click **Save**

### 3.2. Create the Application (OAuth Client)

The application represents the client accessing your MCP server (Claude.ai, your front-end, etc.).

1. Sidebar → **Applications** → **Applications**
2. Click **+ Create Application**
3. Fill in:
   - **Name**: `OWM_MCP_SERVER` (or a descriptive name)
   - **Type**: **Regular Web Application** (confidential client with client_secret)
4. Click **Create**
5. In the **Settings** tab:
   - Note the **Client ID** and **Client Secret** (you will need them for the `.env`)
   - **Allowed Callback URLs**: `https://claude.ai/api/mcp/auth_callback`
     - Add other callback URLs separated by commas if needed
   - **Allowed Web Origins**: `https://<your-ngrok-url>`
   - **Grant Types** (in Advanced Settings → Grant Types): make sure **Authorization Code** and **Refresh Token** are checked
6. Click **Save Changes**

> **Why Regular Web App and not SPA?**
> An SPA is a public client (no secret). Auth0 may not issue a JWT or a refresh_token depending on the configuration.
> With a Regular Web App, the `client_secret` is injected server-side by the `/token` proxy, ensuring a confidential flow.

### 3.3. Authorize the Application to Access the API

1. Sidebar → **Applications** → **APIs**
2. Click on **OWM MCP**
3. **Machine to Machine Applications** tab
4. Find the **OWM_MCP_SERVER** application and **enable it** (toggle ON)
5. Select the required scopes (if you defined any)

---

## 4. Keycloak Setup Step by Step

Keycloak is a self-hosted open-source Identity and Access Management solution. It supports OAuth 2.0 / OIDC natively and can replace Auth0 as the Authorization Server.

> **Prerequisites**: a running Keycloak instance (v20+). You can run one locally with:
>
> ```bash
> docker run -p 8180:8080 -e KC_BOOTSTRAP_ADMIN_USERNAME=admin -e KC_BOOTSTRAP_ADMIN_PASSWORD=admin quay.io/keycloak/keycloak:latest start-dev
> ```
>
> Admin console: `http://localhost:8180`

### 4.1. Create a Realm

A realm is the equivalent of an Auth0 tenant — it isolates users, clients, and configuration.

1. Log in to the Keycloak Admin Console
2. Top-left dropdown → **Create realm**
3. Fill in:
   - **Realm name**: `mcp` (or any name you prefer)
4. Click **Create**

The issuer URL will be: `https://<keycloak-host>/realms/mcp`

### 4.2. Create a Client (Confidential)

The client represents the OAuth application (Claude.ai, your front-end, etc.).

1. Sidebar → **Clients** → **Create client**
2. **General Settings**:
   - **Client type**: `OpenID Connect`
   - **Client ID**: `owm-mcp-client` (this is the `client_id` value)
3. Click **Next**
4. **Capability config**:
   - **Client authentication**: **ON** (this makes it a confidential client)
   - **Authorization**: OFF
   - **Authentication flow**: check **Standard flow** (Authorization Code) and **Direct access grants** (optional, for testing)
5. Click **Next**
6. **Login settings**:
   - **Valid redirect URIs**: `https://claude.ai/api/mcp/auth_callback`
     - Add other callback URLs as needed (one per line)
   - **Valid post logout redirect URIs**: `+` (all)
   - **Web origins**: `https://<your-ngrok-url>`
7. Click **Save**
8. Go to the **Credentials** tab:
   - Note the **Client secret** (you will need it for the `.env`)

### 4.3. Add an Audience Mapper

By default, Keycloak does not include an `aud` (audience) claim matching your resource server identifier. You must add a protocol mapper to include it in the access token.

1. Sidebar → **Client scopes** → **Create client scope**
2. Fill in:
   - **Name**: `owm-mcp-audience`
   - **Type**: `Default`
   - **Protocol**: `OpenID Connect`
3. Click **Save**
4. In the created scope → **Mappers** tab → **Configure a new mapper**
5. Select **Audience**
6. Fill in:
   - **Name**: `owm-mcp-audience-mapper`
   - **Included Custom Audience**: `owm-mcp` (this must match `OAUTH_AUDIENCE`)
   - **Add to ID token**: OFF
   - **Add to access token**: **ON**
7. Click **Save**
8. Assign the scope to your client:
   - Sidebar → **Clients** → `owm-mcp-client` → **Client scopes** tab
   - Click **Add client scope** → select `owm-mcp-audience` → **Add** as **Default**

> **Offline Access (refresh_token)**: Keycloak includes `offline_access` as a built-in optional scope.
> The proxy automatically injects `offline_access` into the authorization request, which triggers Keycloak to return a refresh token.
> No additional configuration is needed on the Keycloak side.

### 4.4. (Optional) Create a User for Testing

1. Sidebar → **Users** → **Add user**
2. Fill in username, email, etc.
3. **Credentials** tab → **Set password** → uncheck "Temporary"
4. Click **Save**

---

## 5. `.env` Configuration

### 5.1. API Key Only

Minimal configuration for local use (VS Code, tests).

```env
X_API_KEY=mysecretkey
```

No need for `OAUTH_*` variables. However `BASE_URL` is still required for Claude Desktop to connect.

### 5.2. OAuth without Proxy

For clients that send the `audience` parameter themselves in `/authorize`.

```env
# Public URL of the server (required for Claude Desktop to connect)
BASE_URL=https://your-domain.example.com

# Authorization Server URL (issuer)
OAUTH_AUTHORIZATION_SERVER=https://your-tenant.auth0.com

# Auth0 API identifier (expected "aud" claim in the JWT)
OAUTH_AUDIENCE=owm-mcp

# Scopes required to access the MCP endpoints (optional, comma-separated)
OAUTH_SCOPES=

# No client_secret needed server-side in this mode
OAUTH_CLIENT_SECRET=
```

> **Note**: in this mode, `OAUTH_AUDIENCE` is used only for JWT validation (`aud` claim).
> The AS proxy is **not** enabled because it depends on `OAUTH_AUDIENCE` combined with `OAUTH_CLIENT_SECRET` in the current code.

### 5.3. OAuth with Proxy (Auth0 + Claude)

Full configuration for clients that do not send `audience` (Claude.ai).

```env
# Public URL of the server (required for Claude Desktop — here behind ngrok)
BASE_URL=https://xxxx.ngrok-free.dev

# Auth0 Authorization Server URL
OAUTH_AUTHORIZATION_SERVER=https://your-tenant.auth0.com

# Auth0 API identifier — injected by the proxy into /authorize
OAUTH_AUDIENCE=owm-mcp

# Required scopes (optional)
OAUTH_SCOPES=

# Client secret of the Auth0 "Regular Web App" application
# Injected by the proxy into /token for the code exchange
OAUTH_CLIENT_SECRET=<your_auth0_app_client_secret>
```

The proxy is **automatically enabled** when `OAUTH_AUDIENCE` is defined (see `main.go`).

The proxy automatically injects:

- `audience=owm-mcp` into `/authorize`
- `scope=offline_access` into `/authorize` (to obtain a `refresh_token`)
- `client_secret` into `/token`

### 5.4. OAuth with Proxy (Keycloak + Claude)

Full configuration for Keycloak as the Authorization Server.

```env
# Public URL of the server (required for Claude Desktop — here behind ngrok)
BASE_URL=https://xxxx.ngrok-free.dev

# Keycloak Authorization Server URL (realm issuer)
OAUTH_AUTHORIZATION_SERVER=https://keycloak.example.com/realms/mcp

# Audience value — must match the audience mapper configured in Keycloak
OAUTH_AUDIENCE=owm-mcp

# Required scopes (optional)
OAUTH_SCOPES=

# Client secret from the Keycloak client's Credentials tab
OAUTH_CLIENT_SECRET=<your_keycloak_client_secret>
```

> **Key differences with Auth0**:
>
> - The `OAUTH_AUTHORIZATION_SERVER` URL includes the `/realms/{realm}` path
> - The JWKS endpoint is automatically discovered at `{issuer}/protocol/openid-connect/certs`
> - Keycloak's issuer always has a trailing slash in some versions — the server normalizes this automatically

### 5.5. API Key + OAuth Combined

Both mechanisms are enabled simultaneously. The server accepts either one:

- If the `Authorization: Bearer ...` header is present → OAuth validation
- Otherwise → `X-API-Key` verification

```env
X_API_KEY=mysecretkey

BASE_URL=https://xxxx.ngrok-free.dev
OAUTH_AUTHORIZATION_SERVER=https://your-tenant.auth0.com
OAUTH_AUDIENCE=owm-mcp
OAUTH_SCOPES=
OAUTH_CLIENT_SECRET=<client_secret>
```

This is the recommended configuration: API key for local development (VS Code), OAuth for remote clients (Claude.ai).

---

## 6. Client Configuration

### 6.1. VS Code / Copilot (API Key)

File `.vscode/mcp.json`:

```json
{
  "servers": {
    "owm": {
      "type": "http",
      "url": "http://localhost:8080/mcp",
      "headers": {
        "X-API-Key": "mysecretkey"
      }
    }
  }
}
```

### 6.2. Claude.ai (OAuth via Proxy)

In Claude.ai → Settings → Integrations → Add MCP Server:

| Field             | Value                                             |
| ----------------- | ------------------------------------------------- |
| **URL**           | `https://xxxx.ngrok-free.dev/mcp`                 |
| **Client ID**     | The Client ID of your Auth0 app (Regular Web App) |
| **Client Secret** | The Client Secret of the same app                 |

---

## 7. Troubleshooting

Enable debug logs to see the details of the OAuth flow:

```env
LOG_LEVEL=DEBUG
```

### Common Errors

| Symptom                                              | Cause                                                     | Solution                                                                   |
| ---------------------------------------------------- | --------------------------------------------------------- | -------------------------------------------------------------------------- |
| `Client is not authorized to access resource server` | The Auth0 app is not authorized on the API                | Auth0 → APIs → your API → Machine to Machine Applications → enable the app |
| `token has invalid issuer`                           | Issuer mismatch in the JWT                                | Verify that `OAUTH_AUTHORIZATION_SERVER` matches the Auth0 domain exactly  |
| `response_keys` without `refresh_token`              | `offline_access` not requested or API does not allow it   | Auth0 → APIs → your API → Settings → enable **Allow Offline Access**       |
| Status 403 after successful authentication           | DNS rebinding protection in the go-sdk                    | Automatically configured when `BASE_URL` is set (proxy enabled)            |
| No requests visible in ngrok                         | The ngrok tunnel is not active or the URL has changed     | Verify that ngrok is running and that the URL in `BASE_URL` matches        |
| `Invalid authorization code` on `/token`             | The code has expired or the `redirect_uri` does not match | Verify the Allowed Callback URLs in Auth0                                  |
