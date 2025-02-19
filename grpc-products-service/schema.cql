CREATE TABLE products (
    id bigint PRIMARY KEY,
    name text,
    description text,
    price float,
    category text,
    tags list<text>,
    stock_count int,
    created_at timestamp,
    updated_at timestamp
);


CREATE TABLE products_outbox (
    bucket text,
    id timeuuid,
    event_type text,
    payload text,
    created_at timestamp,
    PRIMARY KEY (bucket, id)
) WITH CLUSTERING ORDER BY (id ASC);
