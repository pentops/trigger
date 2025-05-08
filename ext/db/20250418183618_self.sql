-- +goose Up
CREATE TABLE selftick (
  selftick_id char(10) NOT NULL,
  data jsonb NOT NULL,
  lasttick timestamp with time zone NOT NULL,
  CONSTRAINT selftick_pk PRIMARY KEY (selftick_id)
);
-- +goose Down
DROP TABLE self;
