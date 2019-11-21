-- +goose Up
-- SQL in this section is executed when the migration is applied.
CREATE TABLE tom_beresford_beer_catalogue
(
    id       SERIAL PRIMARY KEY,
    name     TEXT,
    consumed BOOL DEFAULT TRUE,
    rating   DOUBLE PRECISION
);

-- +goose Down
-- SQL in this section is executed when the migration is rolled back.