# Guia de instalação

Esta página cobre todas as formas de rodar o qLLM: o que cada opção faz, o que exige e como conferir que funcionou. As Releases no GitHub incluem um zip slim (`qllm-standalone-<ver>.zip`); você ainda compila o binário ou a imagem.

## 1. Escolha uma opção

| Opção | Exige | Você recebe | Use quando |
|-------|-------|-------------|------------|
| **A. Imagem de container** (`Dockerfile`) | Docker, nerdctl ou podman | Build completo (DuckDB embutido) com o YAML de `deploy/prd` já dentro | Produção, ou se você não quer toolchain C |
| **B. Build Go, Go puro** | Go 1.26.6+ | `qllm` sem CGO. Sem SQL de catálogo | `validate` rápido, Query IR, CI sem CGO |
| **C. Build Go, DuckDB embutido** (`-tags duckdb`) | Go 1.26.6+, CGO, compilador C (+ `duckdblib` no Windows) | Tudo, inclusive `execute_sql` / `qllm sql` | Desenvolver o produto completo localmente |
| **D. Zip / repo standalone** (asset da Release ou `init-standalone.*`) | Docker (para rodar); Python 3 só se for regenerar | Uma pasta pequena para hospedar no GitHub/GitLab | Sua cópia sem docs, fixtures nem harness |
| **E. Harness Compose** (`docker-compose.yml`) | nerdctl compose (Rancher Desktop) | qLLM + Postgres, MySQL, MongoDB, API fake | Rodar os goldens e testar o demo |
| **F. Simulação Kubernetes** (`deploy/prd-tst`) | Rancher Desktop com Kubernetes | Um cluster estilo fleet-ops | Testar rollouts. Mundo separado, veja [environments.md](environments.md) |
| **G. Demo de chaves com escopo** (`docker-compose.enforced.yml`) | nerdctl/docker compose | qLLM + Postgres + agente LangGraph | Ver o escopo de linha (D21) de ponta a ponta |

Na dúvida: **A** para rodar, **C** para desenvolver.

## 2. Os dois motores: Go puro vs DuckDB embutido

O qLLM tem um motor local de join atrás de uma única API `Engine`. Qual você recebe depende de uma build tag.

| | Go puro (padrão) | DuckDB embutido (`-tags duckdb`) |
|---|---|---|
| Comando | `go build ./cmd/qllm` | `go build -tags duckdb ./cmd/qllm` |
| CGO / compilador C | Não precisa | Obrigatório |
| Query IR (`/v1/queries`, `qllm query`) | Sim, inclusive join entre fontes, agregações REST, `where`, offset | Sim |
| SQL de catálogo (`/v1/sql`, MCP `execute_sql`, `qllm sql`) | **Não**. Não executa o SQL | **Sim** |
| `go test ./...` | Sem CGO | Só os pacotes marcados precisam da tag |

As imagens de container e a produção usam `-tags duckdb`. Se um pedido SQL falha dizendo que falta DuckDB no build, você está com um binário Go puro.

## 3. Opção A: imagem de container

O [`Dockerfile`](../../Dockerfile) da raiz tem dois estágios.

1. **Build** (`golang:1.26.6-bookworm`): instala `gcc` e `libc6-dev`, baixa os módulos Go, define `CGO_ENABLED=1` e roda `go build -tags duckdb`.
2. **Runtime** (`debian:bookworm-slim`): adiciona `ca-certificates`, copia o binário para `/usr/local/bin/qllm` e coloca em `/config` estes arquivos de `deploy/prd/`: `qllm.preset.yaml`, `qllm.catalog.yaml`, `qllm.config.yaml`, `qllm.env.yaml`, `qllm.access.yaml`.

O container sobe com `qllm serve --http --mcp-http --config-dir /config` e expõe **8088** (HTTP `/v1`) e **8089** (MCP HTTP).

```bash
nerdctl build -t qllm .          # ou: docker build -t qllm .
nerdctl run --rm -p 8088:8088 -p 8089:8089 qllm
```

Para usar seu próprio YAML, edite `deploy/prd/default/*.yaml` e reconstrua, ou monte sua pasta sobre a embutida e evite o rebuild:

```bash
docker run --rm -p 8088:8088 -p 8089:8089 -v "$PWD/my-project:/config" qllm
```

Segredos são variáveis de ambiente nomeadas pelas chaves `*Env` do preset, passadas com `-e NOME=valor` ou `--env-file`. Nunca vão no YAML.

Dentro de um container o servidor precisa escutar em um endereço não loopback, o que o `qllm` só permite com auth configurada (ou `serve.insecureBind: true`). Mantenha `serve.addr` e `authTokenEnv` coerentes no `qllm.config.yaml` embutido.

Os três Dockerfiles diferem apenas no que embutem:

| Arquivo | Embute em `/config` | Usado por |
|---------|---------------------|-----------|
| `Dockerfile` | `deploy/prd/default/` | Imagem de produto |
| `Dockerfile.dev` | `deploy/image/config/` | Harness Compose e `prd-tst` (o Kubernetes monta um ConfigMap sobre `/config`) |
| `Dockerfile.enforced` | `deploy/prd/enforced/config/` | Demo de chaves com escopo |

### Rancher Desktop e Kubernetes

O Kubernetes do Rancher Desktop lê imagens do namespace containerd `k8s.io`:

```bash
nerdctl --namespace k8s.io build -t qllm:local .
```

Se você construiu no namespace padrão, copie a imagem:

```bash
nerdctl save qllm:local -o qllm-local.tar
nerdctl --namespace k8s.io load -i qllm-local.tar
```

`ErrImagePull` na simulação significa que a imagem não está nesse namespace (os manifests usam `imagePullPolicy: Never`).

## 4. Opção B: build Go, Go puro

Instale **Go 1.26.6 ou superior** (`go.mod` define `go 1.26.6`).

```bash
go build -o qllm ./cmd/qllm        # Windows: -o qllm.exe
go test ./...
./qllm validate --config-dir ./my-project
```

Nenhum compilador C é usado. Serve para validar arquivos e rodar Query IR. O SQL de catálogo não funciona (seção 2).

## 5. Opção C: build Go com DuckDB embutido

Adicione `-tags duckdb` e habilite o CGO. O resto é o mesmo comando.

### Linux e macOS

Instale um compilador C (`gcc`, ou as ferramentas de linha de comando do Xcode: `xcode-select --install`). Os bindings Go (`duckdb-go-bindings`, veja `go.mod`) incluem a biblioteca DuckDB para linux e darwin em amd64 e arm64, então você não baixa o DuckDB manualmente.

```bash
source ./scripts/dev/dev-shell.sh      # define CGO_ENABLED=1 e as variáveis QLLM_* de demo
go test -tags duckdb ./internal/duckdblocal/
go build -tags duckdb -o qllm ./cmd/qllm
```

O `dev-shell.sh` deve ser **carregado com `source`**, não executado, senão as variáveis somem quando ele termina. As variáveis de demo usam `${VAR:-padrão}`, então o que você já exportou prevalece. Se só precisa do CGO:

```bash
CGO_ENABLED=1 go build -tags duckdb -o qllm ./cmd/qllm
```

### Windows

No Windows o repositório liga com a biblioteca oficial do DuckDB por meio de duas coisas: um gcc MinGW e a pasta `duckdblib/`.

**Passo 1. gcc (MSYS2 UCRT64).** Instale o [MSYS2](https://www.msys2.org/), abra o shell **UCRT64** e rode:

```bash
pacman -S mingw-w64-ucrt-x86_64-gcc
```

O `scripts/dev/dev-shell.ps1` procura `gcc.exe` em `C:\ghcup\msys64\ucrt64\bin`. Se seu MSYS2 está em outro lugar (o padrão é `C:\msys64\ucrt64\bin`), edite a linha `$MsysGccBin` no início do script. Sem gcc o script mostra a dica de instalação e sai.

**Passo 2. Biblioteca DuckDB.** Na [página oficial de instalação do DuckDB](https://duckdb.org/docs/installation/), baixe a biblioteca C/C++ para Windows (`libduckdb-windows-amd64.zip`). Extraia e copie **`duckdb.dll`**, **`duckdb.lib`** e o header **`duckdb.h`** para a pasta `duckdblib/` na raiz do repo. São binários grandes; não os versione no seu fork.

**Passo 3. Carregue o ambiente.** No PowerShell:

```powershell
.\scripts\dev\dev-shell.ps1
```

ou dê duplo clique em `scripts\dev\dev-shell.cmd`, que abre um PowerShell com o script carregado (política de execução ignorada só nessa janela). O script:

| Ação | Motivo |
|------|--------|
| Coloca o `bin` do MSYS2 e `duckdblib/` no início do `PATH` | O linker acha o gcc e o `qllm.exe` acha o `duckdb.dll` **em tempo de execução** |
| `CGO_ENABLED=1`, `CC=gcc` | Liga o CGO |
| `CGO_CFLAGS=-I<duckdblib>` | O compilador acha o `duckdb.h` |
| `CGO_LDFLAGS=-L<duckdblib> -lduckdb` | O linker acha a biblioteca |
| Define variáveis `QLLM_*` de demo (Postgres, MySQL, Mongo e API fake locais) | Casa com o harness Compose. Ignore no seu projeto |
| Entra na raiz do repo e muda o prompt para `qLLM-dev` | Comodidade |

Ele altera só a sessão **atual**. Em um novo terminal você precisa rodá-lo de novo.

**Passo 4. Compile e confira.**

```powershell
go test -tags duckdb ./internal/duckdblocal/
go build -tags duckdb -o qllm.exe ./cmd/qllm
go run -tags duckdb .\scripts\dev\duckdb_smoke.go
```

O `duckdb_smoke.go` abre o motor embutido e executa uma consulta mínima; é a forma mais rápida de provar que CGO e a biblioteca estão ligados corretamente.

Para rodar o `qllm.exe` depois em um terminal comum, deixe o `duckdb.dll` ao lado do executável ou no `PATH`.

## 6. Opção D: zip / repo standalone

Baixe o **`qllm-standalone-<ver>.zip`** na [Release do GitHub](https://github.com/jpandrade30/qLLM/releases), descompacte e faça `docker build` como no README da pasta. Ou gere uma pasta nomeada a partir do clone (sem docs, fixtures nem harness):

```bash
python scripts/standalone/init-standalone.py --user Alice --out ..
./scripts/standalone/init-standalone.sh  --user Alice --out ..      # Linux/macOS, chama python3 ou python
.\scripts\standalone\init-standalone.ps1 --user Alice --out ..      # Windows
```

| Flag | Significado |
|------|-------------|
| `--user` | Obrigatório. O nome vira um slug: minúsculas, o que não for `a-z0-9` vira `-`. Slug que não começa com letra ganha o prefixo `user-` |
| `--out` | Diretório **pai** da nova pasta. Padrão: diretório atual. `--out ..` coloca ao lado deste repo |
| `--force` | Apaga e recria a pasta de destino se existir. Sem ele o script recusa |

O resultado é `<out>/qllm-<slug>/` e o script imprime esse caminho. Contém:

| Caminho | O que é |
|---------|---------|
| `go.mod`, `go.sum`, `cmd/qllm/`, `internal/` | O runtime Go, sem testes |
| `config/` | `qllm.preset.yaml`, `qllm.catalog.yaml`, `qllm.config.yaml`, `qllm.env.yaml`, `qllm.access.yaml` para uma fonte SQLite com uma tabela, `items` |
| `data/app.db` | O arquivo SQLite (`items`: `id=1`, `name=hello`) |
| `Dockerfile` | Mesmo build em dois estágios com `-tags duckdb`, embute `config/` e `data/` |
| `LICENSE.md`, `.env.example`, `.gitignore`, `README.md` | Licença MIT, modelo de token, ignores, instruções de execução |

Rode na pasta nova:

```bash
cp .env.example .env          # defina QLLM_AUTH_TOKEN
docker build -t qllm-alice .
docker run --rm -p 8088:8088 -p 8089:8089 --env-file .env qllm-alice
curl -s http://127.0.0.1:8088/v1/health
```

Sem Docker, um build Go puro roda `validate` (sem SQL). Depois edite preset e catálogo para adicionar suas fontes.

## 7. Opção E: harness Compose

```bash
nerdctl compose up --build
```

Constrói o `Dockerfile.dev` e sobe qLLM, Postgres, MySQL, MongoDB e a API fake. Bearer de demo: `change-me`. Portas no host: HTTP 8088, MCP 8089. Carregue dados com `.\scripts\dev\dev-seed-fake.ps1` ou `./scripts/dev/dev-seed-fake.sh`. Rode os goldens SQL com `pytest fixtures/sqlcheck`. As senhas são fracas e o Mongo é aberto, então use só em localhost. Detalhes: [environments.md](environments.md).

## 8. Opções F e G

- **F.** `.\scripts\prd-tst\prd-tst-up.ps1` (ou `.sh`) constrói `qllm:local` em `k8s.io` e aplica `deploy/prd-tst`. Não rode junto com o Compose. Veja [environments.md](environments.md).
- **G.** Demo de chaves com escopo, com compose próprio: `deploy/prd/enforced/README.md` e [multi-user-safety.md](multi-user-safety.md).

## 9. Depois de instalar

A configuração é **arquivos mais variáveis de ambiente**. Mudar o YAML só pede reinício (ou rollout do Deployment). Recompile apenas quando o código Go mudar.

O mínimo para servir:

1. Uma pasta com `qllm.preset.yaml` e `qllm.catalog.yaml` ([from-scratch.md](from-scratch.md)). Sem eles o `serve` falha com `CONFIG_ERROR`; não há fallback para `fixtures/`.
2. As variáveis de ambiente nomeadas pelas chaves `*Env` do preset.
3. Valide e suba:

```bash
./qllm validate --config-dir ./my-project
./qllm serve --http --mcp-http --config-dir ./my-project
curl -s http://127.0.0.1:8088/v1/health
```

Por padrão escuta em `127.0.0.1:8088` (HTTP) e `127.0.0.1:8089` (MCP). Todos os comandos e flags: [cli.md](cli.md). Auth, bind e CORS: [http-mcp.md](http-mcp.md).

## 10. Solução de problemas

| Sintoma | Causa e correção |
|---------|------------------|
| SQL retorna `UNSUPPORTED` citando DuckDB | Binário Go puro. Recompile com `-tags duckdb` e CGO |
| `gcc not found at …` no `dev-shell.ps1` | Instale o pacote gcc do MSYS2 ou corrija `$MsysGccBin` |
| `Warning: duckdb.dll not found` | Coloque `duckdb.dll`, `duckdb.lib`, `duckdb.h` em `duckdblib/` |
| Build falha com `cannot find -lduckdb` ou `duckdb.h` | `duckdblib/` incompleta, ou você não rodou o `dev-shell.ps1` neste terminal |
| `qllm.exe` sai ao iniciar por DLL ausente | `duckdb.dll` fora do `PATH` e fora da pasta do exe |
| Linux/macOS: `gcc: command not found` | Instale um compilador C |
| `CONFIG_ERROR` no `serve` | Sem `qllm.preset.yaml` / `qllm.catalog.yaml` em `--config-dir` ou no CWD |
| Recusa bind em endereço não loopback | Defina `serve.authTokenEnv` (e exporte a variável) ou `--insecure-bind` só para testes locais |
| Env de auth definida mas o servidor não sobe | A variável nomeada em `authTokenEnv` está vazia |
| `ErrImagePull` na simulação | Construa ou carregue a imagem no namespace `k8s.io` |
| `refusing to overwrite …` no init-standalone | A pasta existe. Use outro `--user` ou `--force` |

Outros detalhes de build (`mcp-go` fixado, quando recompilar): [build.md](build.md).
