1. Start PostgreSQL:

```bash
docker run --name postgres \
  --network bridge \
  -e POSTGRES_PASSWORD=postgres \
  -e POSTGRES_DB=marmot \
  -p 5432:5432 \
  -d postgres:latest
```

2. Start the frontend development server:

```bash
cd web/marmot
pnpm install
pnpm dev
```

3. In another terminal, start the backend:

```bash
make dev
```

The app will be available at:

- Frontend: http://localhost:5173
- Backend API: http://localhost:8080
- API Documentation: http://localhost:8080/swagger/index.html
