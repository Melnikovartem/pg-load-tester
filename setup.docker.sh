#!/bin/bash


export POSTGRES_DSN="host=localhost port=5432 database=postgres user=postgres sslmode=disable password=your_password_here"
# reset db
make migrations.reset
make migrations

# create fake data


# start the sender
