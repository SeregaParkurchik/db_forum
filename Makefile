run:
	docker run -d --name my-forum -p 5001:5000 -m 1g forum-combined

db_fill:
	./technopark-dbms-forum fill --url=http://localhost:5001 --timeout=900

test:
	./technopark-dbms-forum perf --url=http://localhost:5001 --duration=600 --step=60
