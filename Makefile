run:
	cd cmd && go run main.go
test:
	./technopark-dbms-forum func -u http://localhost:8081 -r report.html
clean_db:
	psql "host=localhost port=5555 user=admin password=123456 dbname=postgres sslmode=disable" -c "DROP DATABASE IF EXISTS dbhw;" &
	psql "host=localhost port=5555 user=admin password=123456 dbname=postgres sslmode=disable" -c "CREATE DATABASE dbhw WITH OWNER admin;" &
	psql "host=localhost port=5555 user=admin password=123456 dbname=dbhw sslmode=disable" -f ./migrations/init.sql

create_db:
	./technopark-dbms-forum fill --url=http://localhost:8081 --timeout=900

test_db:
	./technopark-dbms-forum perf --url=http://localhost:8081 --duration=600 --step=60