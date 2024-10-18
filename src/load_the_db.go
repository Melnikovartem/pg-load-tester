package main

import (
	"database/sql"
	"fmt"
	"log"
	"math/rand"
	"os"
	"time"

	"github.com/Masterminds/squirrel"
	_ "github.com/lib/pq"
)

var psql = squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)

func createRequestToPostgres(db *sql.DB, categoryName string) func() error {
	return func() error {
		queryBuilder := psql.Select("*").
			From("games g").
			LeftJoin("game_categories gc ON g.game_id = gc.game_id").
			LeftJoin("categories c ON gc.category_id = c.category_id").
			InnerJoin("game_localization gl ON g.game_id = gl.game_id").
			Where("c.name = ?", categoryName)

		rows, err := queryBuilder.RunWith(db).Query()
		if err != nil {
			return err
		}
		defer rows.Close()
		return nil
	}
}

func main() {
	dsn := os.Getenv("POSTGRES_DSN")
	if dsn == "" {
		log.Fatal("POSTGRES_DSN environment variable not set")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}
	defer db.Close()

	db.SetMaxOpenConns(75)
	db.SetMaxIdleConns(25)
	fmt.Println("connected to database")

	maxSenders := 100_000
	senderInteval := time.Second * 1
	categories := []string{
		"top_playgama",
		"recommended",
		"trending_now",
		"new",
		"io",
		"action",
		"puzzle",
		"racing",
		"hypercasual",
		"arcade",
		"adventure",
		"match3",
		"cards",
		"sports",
		"strategy",
		"simulator",
		"educational",
		"balloons",
		"girls",
		"2_player",
		"boys",
		"tabletop",
		"quiz",
		"role",
		"economic",
		"tests",
		"kids",
		"midcore",
		"horrors",
		"imitations",
	}

	var eventList []func() error
	for _ = range maxSenders {
		randomIndex := rand.Intn(len(categories))
		randomCategory := categories[randomIndex]
		eventList = append(eventList, createRequestToPostgres(db, randomCategory))
	}

	eventSender := NewEventSender(eventList, senderInteval, time.Second*20, time.Second*5)
	eventSender.Start(eventList)

	select {}
}
