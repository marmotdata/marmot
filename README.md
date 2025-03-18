# Marmot
<div style="text-align: center;">
<img src="./marmot.svg" width="200">
</div>

Marmot is an open-source data catalog that helps teams discover, understand, and govern their data assets. It's designed for modern data ecosystems where data flows through multiple systems, formats, and teams.

Marmot was designed with the following in mind:

- **Simplicity**: Single binary deployment
- **Performance**: Fast search and efficient processing
- **Extensibility**: Document almost anything with the flexible API

## Features:

### Powerful Queries:

- Find anything fast with flexible queries.
- Boolean, text, metadata, and comparisons.

### Flexible Integrations

- Connect all your data assets
- CLI, API, Terraform, and Pulumi integrations.

### Lineage Visualization:

- See data flow with interactive graphs.
- Track dependencies and analyze impact.

### Documentation & Governance:

- Markdown documentation support
- Tagging and classification

## Local Development 🛠️

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

## Documentation 📚

See the [docs](./docs) directory for detailed documentation.

## Contributing 🤝

Contributions are welcome! Please feel free to submit a Pull Request.

## License ⚖️

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
