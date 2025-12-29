package main

import (
	"context"
	"fmt"
	"github.com/Adedunmol/answerly/api"
	"github.com/Adedunmol/answerly/database"
	"github.com/Adedunmol/answerly/queue"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"
)

func main() {

	err := godotenv.Load()
	if err != nil {
		log.Printf("error loading .env file: %s. relying on enviroment variables", err)
	}

	ctx := context.Background()

	pool, err := database.ConnectDB(ctx)
	if err != nil {
		log.Fatalf("error connecting to database: %s", err)
	}
	defer pool.Close()

	sqlDB := stdlib.OpenDBFromPool(pool)
	if sqlDB == nil {
		panic("could not unwrap pgxpool to *sql.DB")
	}

	ctxWithTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	q, err := queue.NewClient(ctxWithTimeout)
	if err != nil {
		log.Fatal(fmt.Errorf("error creating new queue client: %w", err))
	}

	queries := database.New(pool)

	r := api.Routes(queries, q, pool)

	port := os.Getenv("PORT")

	if port == "" {
		port = "8080"
	}

	server := &http.Server{Addr: fmt.Sprintf(":%s", port), Handler: r}

	go func() {
		log.Printf("starting web server on po"+
			"rt %s", port)
		if err := server.ListenAndServe(); err != nil {
			log.Fatal(fmt.Errorf("error starting web server on port %s: %w", port, err))
		}
	}()

	go func() {
		if err := q.Run(ctx); err != nil {
			log.Fatal(fmt.Errorf("error starting queue client: %w", err))
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)

	<-stop

	// gracefully shutdown the server after 30 seconds
	ctx, cancel = context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("server forced to shut down: %v", err)
	}

	log.Println("server exited properly")

}
