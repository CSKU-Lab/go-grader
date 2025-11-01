env "local" {
  url = "postgres://go-grader:go-grader-password@localhost:5433/results?sslmode=disable"
  src = "file://atlas.pg.hcl"
}

