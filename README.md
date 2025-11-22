# Zubr Server

## API Endpoints

### Signup
```bash
curl -X POST http://localhost:3000/api/signup \
  -H "Content-Type: application/json" \
  -d '{"username":"yourname","password":"yourpass"}'
```

### Login
```bash
curl -X POST http://localhost:3000/api/login \
  -H "Content-Type: application/json" \
  -d '{"username":"yourname","password":"yourpass"}'
```

### Generate IRC Config
```bash
curl -X POST http://localhost:3000/api/irc/config/generate
```

### Get IRC Config
```bash
curl -X GET http://localhost:3000/api/irc/config
```

### Health Check
```bash
curl http://localhost:3000/api/health
```