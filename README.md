# CRDtoKCL Auto Module converter

## Build

```bash
git clone git@github.com:DavidChevallier/CRDtoKCL.git
cd 
go build -o  main.go
```

## Verwendung

### 1. GitHub URL und Modulname

```bash
./ -url <GitHub-URL> -name <Modulname> [-debug]
```

- `-url`: die GitHub URL des Verzeichnisses
- `-name`: Name von Moduls.
- `-debug`: Debugfoo

**Beispiel:**

```bash
go run . -name "traefik" -url "https://github.com/traefik/traefik-helm-chart/tree/master/traefik/crds" -debug
```

### 2. Verwendung mit einer config

```bash
go run . -config <Pfad zur JSON config> [-debug]
```

- `-config`
- `-debug`

**Beispiel:**

```bash
go run . -config config.json -debug
```

## config

```json
{
    "moduleName": "fooobar",
    "crds": {
        "crd1": "https://raw.githubusercontent.com/DavidChevallier/repo/main/crds/crd1.yaml",
        "crd2": "https://raw.githubusercontent.com/DavidChevallier/repo/main/crds/crd2.yaml"
    }
}
```