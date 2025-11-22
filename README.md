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

### Health Check
```bash
curl http://localhost:3000/api/health
```