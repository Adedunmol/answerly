# To run the migrations against the database
```bash
$ goose up
```

# To rollback the migrations
```bash
$ goose down
```

# To generate Go methods from SQL queries
```bash
$ sqlc generate
```

# Start the application in dev environment with docker compose:
```bash
$ cd answerly
$ docker-compose -f docker-compose.dev.yml up --build
```

# To stop the running containers, use:
```bash
$ docker-compose -f docker-compose.dev.yml down
```