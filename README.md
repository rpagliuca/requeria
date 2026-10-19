# requeria

Coordenação de humanos e agentes de IA orientada por requisitos vivos, com evidência contínua.

Desenho completo em [`docs/arquitetura.md`](docs/arquitetura.md).

## Estado atual

Prova de conceito do backend de **Tarefas** e **Sessões de Trabalho**: API HTTP sobre SQLite, com o
contrato em [`api/openapi.yaml`](api/openapi.yaml) e as invariantes impostas pela API e pelo banco.

## Rodar

```bash
go run ./cmd/requeria -banco requeria.db -endereco 127.0.0.1:8787
```

## Desenvolver

```bash
go generate ./api   # regenera api/api.gen.go a partir do contrato OpenAPI
go test ./...
```
