# Manual de implementación de qLLM

**¿Vas a escribir todos los archivos `qllm.*` a mano, desde cero?** Empieza por [from-scratch.md](from-scratch.md) y [field-reference.md](field-reference.md). Hay una carpeta de ejemplo en [`deploy/prd/`](../../deploy/prd); consulta también [point-your-folder.md](point-your-folder.md). La simulación en Kubernetes está en `deploy/prd-tst`.

El contrato normativo (JSON Schema, Query IR, errores) sigue en [`planning/`](../../planning/) y [`planning/schemas/`](../../planning/schemas/). Este manual es la guía operativa. Si ambos difieren, **prevalece `planning/`**.

| Documento | Contenido |
|-----------|-----------|
| [from-scratch.md](from-scratch.md) | Tu propia carpeta, un ejemplo con Postgres, `validate` y cómo **comprobar** que se cargó el YAML correcto |
| [field-reference.md](field-reference.md) | Todos los campos, enums, claves `*Env` y opciones REST |
| [point-your-folder.md](point-your-folder.md) | CLI, `Dockerfile` vs `.dev`, `deploy/prd` vs `prd-tst` |
| [scope.md](scope.md) | Qué es el producto y qué no es |
| [project-files.md](project-files.md) | Descubrimiento de archivos y precedencia |
| [cli.md](cli.md) | Todos los comandos y flags |
| [http-mcp.md](http-mcp.md) | HTTP `/v1`, MCP, autenticación, bind y CORS |
| [responses.md](responses.md) | Sobre de salida, tipos de columna, celdas json, `shape` |
| [queries.md](queries.md) | SQL de catálogo vs Query IR: qué se acepta y qué se rechaza |
| [connectors.md](connectors.md) | Tipos de fuente, bindings y pushdown |
| [errors.md](errors.md) | Códigos de error tipados |
| [multi-user-safety.md](multi-user-safety.md) | Alcance por fila en la credencial (D21) |
| [environments.md](environments.md) | Compose, imágenes, simulación PRD y scripts |
| [install.md](install.md) | Todas las opciones de instalación: imagen, Go, DuckDB (`duckdblib` en Windows), standalone |
| [build.md](build.md) | Go, `-tags duckdb` y Docker |

Lee primero [scope.md](scope.md) y [project-files.md](project-files.md). Sin un preset y un catálogo válidos, el binario falla con `CONFIG_ERROR`; no hay ningún respaldo hacia `fixtures/`.

Otros idiomas: [English](../en/README.md) · [Português](../pt/README.md) · [中文](../zh/README.md)
