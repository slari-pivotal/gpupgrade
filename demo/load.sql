drop table if exists t1;
create table t1 (a int) distributed randomly;
insert into t1 select generate_series(1, 1000);
