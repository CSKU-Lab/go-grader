schema "public" {
  comment = "standard public schema"
}

enum "execution_status" {
  schema = schema.public
  values = [
    "COMPILE_PASSED",
    "COMPILE_FAILED",
    "RUN_PASSED",
    "RUN_FAILED",
    "TIME_LIMIT_EXCEEDED",
    "MEMORY_LIMIT_EXCEEDED",
    "RUNTIME_ERROR",
    "SIGNAL_ERROR",
    "GRADER_ERROR",
  ]
}

table "run_results" {
  schema = schema.public
  column "id" {
    type = uuid
  }

  column "status" {
    type = enum.execution_status
  }

  column "output" {
    type = text
  }

  column "wall_time" {
    type = float
  }

  column "memory" {
    type = int
  }

  primary_key {
    columns = [ column.id ]
  }
}

table "grade_results" {
  schema = schema.public
  column "id" {
    type = uuid
  }

  column "status" {
    type = enum.execution_status
  }

  column "error" {
    type = text
  }

  primary_key {
    columns = [ column.id ]
  }
}

table "test_case_results" {
  schema = schema.public
  column "test_case_id" {
    type = uuid
  }

  column "grade_result_id" {
    type = uuid
  }

  column "status" {
    type = enum.execution_status
  }

  column "output" {
    type = text
  }

  column "message" {
    type = text
  }

  column "wall_time" {
    type = float
  }

  column "memory" {
    type = int
  }

  primary_key {
    columns = [ column.test_case_id, column.grade_result_id ]
  }

  foreign_key "fk_grade_result" {
    columns = [ column.grade_result_id ]
    ref_columns = [ table.grade_results.column.id ]
    on_delete = "CASCADE"
  }
}
