create table if not exists test (
    id serial not null,
    name varchar(64) not null,
    created_at timestamptz default now(),
    constraint test_pk primary key (id),
    constraint test_name_unique unique (name)
);
