-- +goose Up

CREATE TABLE trigger (
  trigger_id char(22),
  state jsonb NOT NULL,
  CONSTRAINT trigger_pk PRIMARY KEY (trigger_id)
);

CREATE TABLE trigger_event (
  id uuid,
  trigger_id char(22) NOT NULL,
  timestamp timestamptz NOT NULL,
  sequence int NOT NULL,
  data jsonb NOT NULL,
  state jsonb NOT NULL,
  CONSTRAINT trigger_event_pk PRIMARY KEY (id),
  CONSTRAINT trigger_event_fk_state FOREIGN KEY (trigger_id) REFERENCES trigger(trigger_id)
);

-- +goose Down

DROP TABLE trigger_event;
DROP TABLE trigger;
