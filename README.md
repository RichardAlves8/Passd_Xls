# ChangePass XLS

Ferramenta de linha de comando escrita em Go para **auditar e rotacionar credenciais OLE DB embutidas em arquivos Excel** (`.xls`, `.xlsx`, `.xlsm`) de forma paralela e em larga escala.

---

## Origem

Este projeto nasceu de um problema organizacional real: um servidor de arquivos compartilhado pela empresa continha **milhares de planilhas Excel**, e parte delas havia sido criada com conexões diretas ao banco de dados usando a **senha mestra**. O problema era que não havia como saber quais arquivos continham essa credencial sem abrir um por um manualmente.

Para resolver isso, foi criado este script — capaz de varrer toda a estrutura de diretórios, identificar exatamente quais planilhas possuem a credencial antiga e, quando necessário, substituí-la pela nova, tudo isso sem corromper os arquivos.

---

## Como funciona

Arquivos `.xlsx` e `.xlsm` são internamente arquivos ZIP. Dentro deles existe o arquivo `xl/connections.xml`, que armazena as strings de conexão OLE DB — incluindo usuário e senha em texto claro.

O programa:

1. Varre os diretórios informados em busca de arquivos Excel
2. Abre cada arquivo como ZIP e inspeciona o `xl/connections.xml`
3. Detecta se a credencial antiga está presente
4. Em modo **somente leitura**: apenas reporta quais arquivos contêm a credencial
5. Em modo **escrita**: reescreve o ZIP substituindo a credencial pela nova
6. Gera relatórios CSV e um log detalhado de todas as operações

---

## Pré-requisitos

- [Go](https://golang.org/) 1.20 ou superior
- Arquivo `.env` configurado (veja abaixo)

---

## Configuração

Crie um arquivo `.env` na raiz do projeto com as credenciais:

```env
OLD_UID=usuario_antigo
OLD_PWD=senha_antiga
NEW_UID=usuario_novo
NEW_PWD=senha_nova
```

> Variáveis de ambiente já definidas no sistema têm prioridade sobre o `.env`.

---

## Compilação

```bash
go build -o changepass .
```

---

## Uso

```bash
./changepass <diretório1> [diretório2] [diretório3] ...
```

**Exemplos:**

```bash
# Varrer um único diretório
./changepass /mnt/servidor/Financeiro

./changepass /mnt/servidor/Financeiro /mnt/servidor/RH /mnt/servidor/Operacional
```

---

## Saídas geradas

| Arquivo               | Descrição                                                                 |
|-----------------------|---------------------------------------------------------------------------|
| `Csv/relat.csv`       | Relatório detalhado por pasta e extensão com contagem de arquivos e senhas encontradas |
| `Csv/register.csv`    | Histórico acumulado de execuções (append), agrupado por volume e raiz     |
| `Log/OLE.log`         | Log completo de cada arquivo processado, com erros e substituições        |

Os diretórios `Csv/` e `Log/` são criados automaticamente se não existirem.

---

## Modos de operação

O modo de operação é controlado pelo parâmetro `readonly` na função `changeOLE`:

| Modo | Comportamento |
|------|---------------|
| `readonly = true` *(padrão no `main`)* | Apenas detecta e reporta arquivos com a credencial antiga. Nenhum arquivo é modificado. |
| `readonly = false` | Substitui a credencial e reescreve o arquivo Excel no lugar. |

Para ativar o modo de escrita, altere a chamada em `Analityc.go`:

```go
hasPass := changeOLE(a, false) // false = modo escrita
```

---

## Paralelismo

O processamento ocorre em paralelo com **8 workers** por padrão. Para ajustar, altere a linha marcada com `@CHANGE WORKERS` em `Analityc.go`:

```go
sem := make(chan struct{}, 8) // @CHANGE WORKERS
```

---

## Estrutura do projeto

```
.
├── Analityc.go   # Ponto de entrada, varredura, relatórios CSV
├── OLEScript.go  # Lógica de inspeção e modificação dos arquivos Excel
├── .env          # Credenciais (não versionar)
└── go.mod
```

---

## Segurança

- **Nunca versione o `.env`** — adicione-o ao `.gitignore`
- Variáveis de ambiente do sistema sobrescrevem o `.env`, permitindo uso seguro em pipelines de CI/CD
