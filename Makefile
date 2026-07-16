.PHONY: integration integration-up integration-down integration-test

# Brings up Postgres/MySQL/SQL Server via docker-compose, waits for them to be
# healthy, creates the SQL Server test database (its image doesn't support
# docker-entrypoint-initdb.d), runs the integration-tagged test suite against
# all three, then tears the containers down.
integration: integration-up integration-test integration-down

integration-up:
	docker compose up -d --wait
	docker compose exec -T mssql /opt/mssql-tools18/bin/sqlcmd -C -S localhost -U sa -P "TestPass123!" \
		-Q "IF DB_ID('testdb') IS NULL CREATE DATABASE testdb;"

integration-test:
	MCP_DB_IT_POSTGRES_DSN="postgres://testuser:testpass@localhost:55432/testdb?sslmode=disable" \
	MCP_DB_IT_MYSQL_DSN="testuser:testpass@tcp(localhost:53306)/testdb" \
	MCP_DB_IT_MSSQL_DSN="sqlserver://sa:TestPass123!@localhost:51433?database=testdb" \
	go test -tags integration -v ./integration/...

integration-down:
	docker compose down -v
