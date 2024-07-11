# CRDtoKCL Auto Module converter

## Build

```bash
git clone git@github.com:DavidChevallier/CRDtoKCL.git
```

## Usage

### 1. GitHub URL and Module Name

```bash
./ -url <GitHub-URL> -name <Modulname> [-debug]
```

- `-url`: The GitHub URL of the directory.
- `-name`: Name of the module.
- `-debug`: Enable debugging.

**Example:**

```bash
go run . -name "traefik" -url "https://github.com/traefik/traefik-helm-chart/tree/master/traefik/crds" -debug
```

### 2. Usage with a config file

```bash
go run . -config <Pfad zur JSON config> [-debug]
```

- `-config`
- `-debug`

**Example:**

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
