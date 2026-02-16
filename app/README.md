# Uso de `.env` (desarrollo)

Guía rápida para usar variables de entorno en desarrollo:

- Copia el ejemplo y edita tus valores:

  - Windows (CMD):
  ```bash
  copy .env.example .env
  ```

  - PowerShell:
  ```bash
  Copy-Item .env.example .env
  ```

  - Linux / macOS:
  ```bash
  cp .env.example .env
  ```

- Edita `app/.env` con tus credenciales (Postgres, Redis, etc.).
- No subas `app/.env` al repositorio — ya está en `.gitignore`.
- El binario carga automáticamente `.env` en desarrollo (usa `godotenv`).

- Para instalar dependencias y ejecutar la aplicación:
```bash
cd app
go get ./...
go run .
```

- Ejecutar con una variable temporal:
  - CMD:
  ```bash
  set PORT=9090 && go run .
  ```
  - PowerShell:
  ```bash
  $env:PORT=9090; go run .
  ```
  - Linux/macOS:
  ```bash
  PORT=9090 go run .
  ```

Fin.
