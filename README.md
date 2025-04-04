# Marmot

<div style="text-align: center;">
<img src="./marmot.svg" width="200">
</div>

> ❗ This project is currently in active development and subject to change. It's not recommended you deploy this in production. You may encounter bugs and data may not be accurate.

Marmot is an open-source data catalog that helps teams discover, understand, and govern their data assets. It's designed for modern data ecosystems where data flows through multiple systems, formats, and teams.

Marmot was designed with the following in mind:

- **Simplicity**: Easy to use UI, single binary deployment
- **Performance**: Fast search and efficient processing
- **Extensibility**: Document almost anything with the flexible API

## ✨ Features:

- **Find anything fast with flexible queries**: Boolean, text, metadata, and comparison operators for powerful search capabilities
- **Populate assets with your preferred technology:** CLI, API, Terraform, and Pulumi integrations
- **Visualize data flows with interactive graphs:** Track dependencies and analyze impact through comprehensive lineage visualization
- **Documentation and governance:** Markdown documentation support with tagging and classification capabilities

## 🔌Supported Data Sources

Marmot offers multiple ways to ingest assets: through its CLI, API, or infrastructure-as-code solutions like Terraform and Pulumi. The CLI features a growing ecosystem of plugins with current support for Kafka, AsyncAPI, PostgreSQL, SQS, SNS, and many others. For assets not covered by existing plugins, Marmot's flexible API and infrastructure-as-code providers enable ingestion of virtually any asset type you need.

## 📚 Documentation

If you want to get started with Marmot quickly following the [quickstart guide in the documentation](https://marmotdata.io/docs/quick-start) to get up and running in minutes.

You can also check out the [API documentation](https://marmotdata.io/api) to help with building custom integrations.

## 🛠️ Local Development

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

## 🤝 Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## ⚖️License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
