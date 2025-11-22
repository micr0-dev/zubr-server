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

### IRC Configuration

#### `POST /api/irc/config/generate`
Generate InspIRCd configuration file. This creates the necessary config for the IRC server based on your domain settings.

// TODO will be more extensive later

**Request Body:** None

**Response (200 OK):**
```json
{
  "success": true,
  "message": "InspIRCd configuration generated successfully"
}
```

**Example:**
```bash
curl -X POST http://localhost:3000/api/irc/config/generate
```

**Error Responses:**
- `500 Internal Server Error` - Failed to generate config
```json
{
  "success": false,
  "message": "Failed to generate IRC config: <error details>"
}
```

---

#### `GET /api/irc/config`
Retrieve the current InspIRCd configuration.

**Request Body:** None

**Response (200 OK):**
```json
{
  "success": true,
  "message": "IRC configuration retrieved successfully",
  "config": "<config format=\"xml\">...</config>"
}
```

**Example:**
```bash
curl -X GET http://localhost:3000/api/irc/config
```

**Error Responses:**
- `404 Not Found` - Config file doesn't exist yet
```json
{
  "success": false,
  "message": "IRC config not found. Generate it first using POST /api/irc/config/generate"
}
```

---

### System

#### `GET /api/health`
Health check endpoint.

**Request Body:** None

**Response (200 OK):**
```
OK
```

**Example:**
```bash
curl http://localhost:3000/api/health
```