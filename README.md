# Zubr Server

A decentralized IRC-powered chat application backend.

## API Endpoints

### Authentication

#### `POST /api/signup`
Create a new user account.

**Request Body:**
```json
{
  "username": "string (required)",
  "password": "string (required)",
  "email": "string (optional)"
}
```

**Response (200 OK):**
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "username": "yourname"
}
```

**Example:**
```bash
curl -X POST http://localhost:3000/api/signup \
  -H "Content-Type: application/json" \
  -d '{"username":"alice","password":"secure123","email":"alice@example.com"}'
```

**Error Responses:**
- `400 Bad Request` - Missing username or password
- `409 Conflict` - Username already taken
- `500 Internal Server Error` - Server error

---

#### `POST /api/login`
Authenticate an existing user.

**Request Body:**
```json
{
  "username": "string (required)",
  "password": "string (required)"
}
```

**Response (200 OK):**
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "username": "yourname"
}
```

**Example:**
```bash
curl -X POST http://localhost:3000/api/login \
  -H "Content-Type: application/json" \
  -d '{"username":"alice","password":"secure123"}'
```

**Error Responses:**
- `400 Bad Request` - Invalid request format
- `401 Unauthorized` - Invalid credentials
- `403 Forbidden` - Account not active
- `500 Internal Server Error` - Server error

---

#### `GET /api/user/me`
Get the current authenticated user's information.

**Authentication:** Required (JWT Bearer token)

**Request Headers:**
```
Authorization: Bearer <jwt-token>
```

**Request Body:** None

**Response (200 OK):**
```json
{
  "id": "20251123032944",
  "username": "micr0",
  "email": "micr0@example.com",
  "created_at": "2025-11-23T03:29:44+10:30",
  "active": true,
  "banned": false,
  "role": "owner"
}
```

**Example:**
```bash
curl -X GET http://localhost:3000/api/user/me \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
```

**Error Responses:**
- `401 Unauthorized` - Missing or invalid token
- `404 Not Found` - User not found

**Note:** Password hash is never included for security reasons.

---

### User Management

#### `GET /api/users`
Retrieve list of all users (authenticated).

**Authentication:** Required (JWT Bearer token)

**Request Headers:**
```
Authorization: Bearer <jwt-token>
```

**Request Body:** None

**Response (200 OK):**
```json
{
  "users": [
    {
      "id": "20251123032944",
      "username": "micr0",
      "email": "micr0@example.com",
      "created_at": "2025-11-23T03:29:44+10:30",
      "active": true,
      "role": "owner"
    },
    {
      "id": "20251123040102",
      "username": "alice",
      "email": "alice@example.com",
      "created_at": "2025-11-23T04:01:02+10:30",
      "active": true,
      "role": "user"
    }
  ],
  "total": 2
}
```

**Example:**
```bash
curl -X GET http://localhost:3000/api/users \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
```

**Error Responses:**
- `401 Unauthorized` - Missing or invalid token

**Note:** Password hashes are never returned for security reasons.

---

### Admin Endpoints

**Authorization:** All admin endpoints require the user to be an Owner or Admin.

**Permission Rules:**
- Owner can perform any action
- Admins cannot demote, ban, kick, or delete other admins or the owner
- Regular users cannot access admin endpoints

#### `POST /api/admin/users/:username/promote`
Promote a user to admin role.

**Authentication:** Required (JWT Bearer token)
**Authorization:** Owner or Admin

**Request Headers:**
```
Authorization: Bearer <jwt-token>
```

**Response (200 OK):**
```json
{
  "success": true,
  "message": "User promoted to admin"
}
```

**Example:**
```bash
curl -X POST http://localhost:3000/api/admin/users/alice/promote \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
```

**Error Responses:**
- `400 Bad Request` - Invalid user ID
- `401 Unauthorized` - Missing or invalid token
- `403 Forbidden` - Insufficient permissions
- `404 Not Found` - User not found

---

#### `POST /api/admin/users/:username/demote`
Demote an admin to regular user role.

**Authentication:** Required (JWT Bearer token)
**Authorization:** Owner or Admin (only Owner can demote admins)

**Request Headers:**
```
Authorization: Bearer <jwt-token>
```

**Response (200 OK):**
```json
{
  "success": true,
  "message": "User demoted to regular user"
}
```

**Example:**
```bash
curl -X POST http://localhost:3000/api/admin/users/alice/demote \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
```

**Error Responses:**
- `400 Bad Request` - Invalid user ID
- `401 Unauthorized` - Missing or invalid token
- `403 Forbidden` - Insufficient permissions (only owner can demote admins)
- `404 Not Found` - User not found

**Permission Note:** Admins cannot demote other admins; only the owner can demote admins.

---

#### `POST /api/admin/users/:username/ban`
Ban a user from the server (prevents login and removes from IRC).

**Authentication:** Required (JWT Bearer token)
**Authorization:** Owner or Admin (only Owner can ban admins)

**Request Headers:**
```
Authorization: Bearer <jwt-token>
```

**Response (200 OK):**
```json
{
  "success": true,
  "message": "User banned successfully"
}
```

**Example:**
```bash
curl -X POST http://localhost:3000/api/admin/users/alice/ban \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
```

**Error Responses:**
- `400 Bad Request` - Invalid user ID
- `401 Unauthorized` - Missing or invalid token
- `403 Forbidden` - Insufficient permissions (only owner can ban admins)
- `404 Not Found` - User not found

**Note:** Banning a user also removes them from the IRC server.

---

#### `POST /api/admin/users/:username/kick`
Kick a user from the IRC server (temporary - they can reconnect).

**Authentication:** Required (JWT Bearer token)
**Authorization:** Owner or Admin (only Owner can kick admins)

**Request Headers:**
```
Authorization: Bearer <jwt-token>
```

**Response (200 OK):**
```json
{
  "success": true,
  "message": "User kicked from IRC server (they can reconnect)"
}
```

**Example:**
```bash
curl -X POST http://localhost:3000/api/admin/users/alice/kick \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
```

**Error Responses:**
- `400 Bad Request` - Invalid user ID
- `401 Unauthorized` - Missing or invalid token
- `403 Forbidden` - Insufficient permissions (only owner can kick admins)
- `404 Not Found` - User not found

**Note:** This is a temporary action. The user can reconnect to IRC after being kicked.

---

#### `DELETE /api/admin/users/:username`
Permanently delete a user account.

**Authentication:** Required (JWT Bearer token)
**Authorization:** Owner or Admin (only Owner can delete admins)

**Request Headers:**
```
Authorization: Bearer <jwt-token>
```

**Response (200 OK):**
```json
{
  "success": true,
  "message": "User account deleted successfully"
}
```

**Example:**
```bash
curl -X DELETE http://localhost:3000/api/admin/users/alice \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
```

**Error Responses:**
- `400 Bad Request` - Invalid user ID
- `401 Unauthorized` - Missing or invalid token
- `403 Forbidden` - Insufficient permissions (only owner can delete admins)
- `404 Not Found` - User not found

**Warning:** This action is permanent and cannot be undone. The user's account and IRC access will be completely removed.

---

#### `GET /api/admin/users/:username/permissions`
Check what actions the authenticated user can perform on a target user.

**Authentication:** Required (JWT Bearer token)
**Authorization:** Owner or Admin

**Request Headers:**
```
Authorization: Bearer <jwt-token>
```

**Response (200 OK) - Owner checking regular user:**
```json
{
  "target_user": "alice",
  "target_role": "user",
  "can_promote": true,
  "can_demote": false,
  "can_ban": true,
  "can_kick": true,
  "can_delete": true
}
```

**Response (200 OK) - Admin checking another admin:**
```json
{
  "target_user": "bob",
  "target_role": "admin",
  "can_promote": false,
  "can_demote": false,
  "can_ban": false,
  "can_kick": false,
  "can_delete": false
}
```

**Response (200 OK) - Owner checking admin:**
```json
{
  "target_user": "bob",
  "target_role": "admin",
  "can_promote": false,
  "can_demote": true,
  "can_ban": true,
  "can_kick": true,
  "can_delete": true
}
```

**Example:**
```bash
curl -X GET http://localhost:3000/api/admin/users/alice/permissions \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
```

**Error Responses:**
- `400 Bad Request` - Invalid user ID
- `401 Unauthorized` - Missing or invalid token
- `403 Forbidden` - Insufficient permissions (must be owner or admin)
- `404 Not Found` - User not found

**Use Case:** This endpoint is useful for frontend applications to determine which action buttons to display or enable for a given user in the UI.

---

### User Configuration

#### `GET /api/user/config`
Retrieve the authenticated user's full configuration (for stateless client).

**Authentication:** Required (JWT Bearer token)

**Request Headers:**
```
Authorization: Bearer <jwt-token>
```

**Request Body:** None

**Response (200 OK):**
```json
{
  "log": false,
  "awayMessage": "",
  "clientSettings": {},
  "networks": []
}
```

**Example:**
```bash
curl -X GET http://localhost:3000/api/user/config \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
```

**Error Responses:**
- `401 Unauthorized` - Missing or invalid token
- `404 Not Found` - User not found
- `500 Internal Server Error` - Server error

---

#### `PUT /api/user/config`
Save the authenticated user's full configuration.

**Authentication:** Required (JWT Bearer token)

**Request Headers:**
```
Authorization: Bearer <jwt-token>
```

**Request Body:**
```json
{
  "log": true,
  "awayMessage": "",
  "clientSettings": {
    "theme": "dark",
    "notifications": true
  },
  "networks": [
    {
      "uuid": "550e8400-e29b-41d4-a716-446655440000",
      "name": "Home Server",
      "host": "127.0.0.1",
      "port": 6667,
      "tls": false,
      "rejectUnauthorized": true,
      "password": "",
      "nick": "username",
      "username": "username",
      "realname": "username",
      "sasl": "",
      "saslAccount": "",
      "saslPassword": "",
      "commands": [],
      "awayMessage": "",
      "leaveMessage": "",
      "channels": [
        {
          "name": "#general",
          "key": "",
          "muted": false
        }
      ],
      "proxyEnabled": false,
      "proxyHost": "",
      "proxyPort": 1080,
      "proxyUsername": "",
      "proxyPassword": "",
      "userDisconnected": false,
      "highlightRegex": "",
      "ignoreList": []
    }
  ]
}
```

**Response (200 OK):**
```json
{
  "success": true,
  "message": "Configuration updated successfully"
}
```

**Example:**
```bash
curl -X PUT http://localhost:3000/api/user/config \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..." \
  -d '{"log":true,"clientSettings":{},"networks":[]}'
```

**Error Responses:**
- `400 Bad Request` - Invalid request format
- `401 Unauthorized` - Missing or invalid token
- `404 Not Found` - User not found
- `500 Internal Server Error` - Server error

---

### Instance Settings

#### `GET /api/instance/settings`
Get current instance settings.

**Authentication:** Required (JWT Bearer token)
**Authorization:** Owner or Admin

**Request Headers:**
```
Authorization: Bearer <jwt-token>
```

**Request Body:** None

**Response (200 OK):**
```json
{
  "signup_mode": "public",
  "motd": "Welcome to micr0.dev! Please be nice.",
  "domain": "micr0.dev",
  "network_name": "micr0net"
}
```

**Example:**
```bash
curl -X GET http://localhost:3000/api/instance/settings \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
```

**Error Responses:**
- `401 Unauthorized` - Missing or invalid token
- `403 Forbidden` - Insufficient permissions (must be owner or admin)

**Settings Fields:**
- `signup_mode` - Controls user registration: `"public"`, `"approval"`, or `"invite"`
- `motd` - Message of the day shown to users
- `domain` - Server domain name
- `network_name` - IRC network name

---

#### `PATCH /api/instance/settings`
Update instance settings (partial update).

**Authentication:** Required (JWT Bearer token)
**Authorization:** Owner or Admin

**Request Headers:**
```
Authorization: Bearer <jwt-token>
```

**Request Body (partial updates allowed):**
```json
{
  "signup_mode": "approval",
  "motd": "New welcome message!"
}
```

**Response (200 OK):**
```json
{
  "signup_mode": "approval",
  "motd": "New welcome message!",
  "domain": "micr0.dev",
  "network_name": "micr0net"
}
```

**Example:**
```bash
curl -X PATCH http://localhost:3000/api/instance/settings \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..." \
  -d '{"signup_mode":"approval","motd":"New message"}'
```

**Error Responses:**
- `400 Bad Request` - Invalid request format or invalid signup_mode value
- `401 Unauthorized` - Missing or invalid token
- `403 Forbidden` - Insufficient permissions (must be owner or admin)
- `500 Internal Server Error` - Failed to save settings

**Valid `signup_mode` values:**
- `"public"` - Anyone can sign up (default)
- `"approval"` - Signups require admin approval (not yet implemented)
- `"invite"` - Users can only join via invite (not yet implemented)

**Note:** You can update any combination of fields. Only the fields you include will be updated.

---

### System

#### `GET /api/info`
Get server information including name and version.

**Request Body:** None

**Response (200 OK):**
```json
{
  "name": "Zubr Server",
  "version": "0.1.0",
  "api": "v1"
}
```

**Example:**
```bash
curl http://localhost:3000/api/info
```

---

#### `GET /api/health`
Comprehensive health check endpoint that monitors all system components.

**Request Body:** None

**Response (200 OK - Healthy):**
```json
{
  "status": "healthy",
  "timestamp": "2025-11-23T08:30:00Z",
  "components": {
    "api": {
      "status": "up",
      "message": "API server is running"
    },
    "storage": {
      "status": "up",
      "message": "Storage operational",
      "latency": "245.3µs"
    },
    "irc_database": {
      "status": "up",
      "message": "IRC database operational",
      "latency": "1.2ms"
    },
    "inspircd": {
      "status": "up",
      "message": "InspIRCd running and accepting connections",
      "latency": "3.5ms"
    }
  }
}
```

**Response (200 OK - Degraded):**
```json
{
  "status": "degraded",
  "timestamp": "2025-11-23T08:30:00Z",
  "components": {
    "api": {
      "status": "up",
      "message": "API server is running"
    },
    "storage": {
      "status": "up",
      "message": "Storage operational",
      "latency": "245.3µs"
    },
    "irc_database": {
      "status": "up",
      "message": "IRC database operational",
      "latency": "1.2ms"
    },
    "inspircd": {
      "status": "degraded",
      "message": "InspIRCd process running but port 6667 not responding",
      "latency": "2.1s"
    }
  }
}
```

**Response (503 Service Unavailable - Unhealthy):**
```json
{
  "status": "unhealthy",
  "timestamp": "2025-11-23T08:30:00Z",
  "components": {
    "api": {
      "status": "up",
      "message": "API server is running"
    },
    "storage": {
      "status": "up",
      "message": "Storage operational",
      "latency": "245.3µs"
    },
    "irc_database": {
      "status": "up",
      "message": "IRC database operational",
      "latency": "1.2ms"
    },
    "inspircd": {
      "status": "down",
      "message": "InspIRCd process not running"
    }
  }
}
```

**Component Status Values:**
- `up` - Component is fully operational
- `degraded` - Component is running but experiencing issues
- `down` - Component is not functioning

**Overall Status:**
- `healthy` - All components are up
- `degraded` - At least one component is degraded (returns 200)
- `unhealthy` - At least one component is down (returns 503)

**Example:**
```bash
curl http://localhost:3000/api/health
```

**Monitored Components:**
- **api** - API server status (always up if endpoint responds)
- **storage** - User data storage (JSON file access)
- **irc_database** - IRC authentication database (SQLite)
- **inspircd** - InspIRCd process and port 6667 connectivity