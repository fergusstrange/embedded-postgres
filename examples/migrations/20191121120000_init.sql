-- +goose Up
-- SQL in this section is executed when the migration is applied.
CREATE TABLE beer_catalogue
(
    id       SERIAL PRIMARY KEY,
    name     TEXT,
    consumed BOOL DEFAULT TRUE,
    rating   DOUBLE PRECISION
);

INSERT INTO beer_catalogue (name, consumed, rating)
VALUES ('Punk IPA', true, 68.29);

-- +goose Down
-- SQL in this section is executed when the migration is rolled back.