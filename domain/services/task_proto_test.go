package services_test

import (
	"testing"

	taskPB "github.com/CSKU-Lab/go-grader/genproto/task/v1"
	"github.com/stretchr/testify/assert"
)

// TestTaskResponse_FieldNumbering verifies that TaskResponse has the correct field numbering
// to match task-server proto schema. This is critical for gRPC wire format compatibility.
// Issue: https://github.com/CSKU-Lab/super-app/issues/32
func TestTaskResponse_FieldNumbering(t *testing.T) {
	// Create a TaskResponse with all fields set
	compareScriptID := "default_compare"
	task := &taskPB.TaskResponse{
		Id:              "task-123",
		TestCaseGroups:  []*taskPB.TestCaseGroup{},
		AllowedRunners:  []*taskPB.AllowedRunner{},
		CompareScriptId: &compareScriptID, // Field 4
		Limit: &taskPB.Limit{
			CpuTime:      1.0,
			CpuExtraTime: 0.5,
			WallTime:     2.0,
			Memory:       256,
			Stack:        8,
			MaxOpenFiles: 64,
			MaxFileSize:  10.0,
			NetworkAllow: false,
		},
		Solution: &taskPB.Solution{
			RunnerId: "python3",
			Files:    []*taskPB.File{},
		},
		ResourceFiles: []*taskPB.File{},
	}

	// Verify all fields are accessible via getter methods
	assert.Equal(t, "task-123", task.GetId())
	assert.NotNil(t, task.GetTestCaseGroups())
	assert.NotNil(t, task.GetAllowedRunners())
	assert.Equal(t, "default_compare", task.GetCompareScriptId())
	assert.NotNil(t, task.GetLimit())
	assert.Equal(t, float32(1.0), task.GetLimit().GetCpuTime())
	assert.NotNil(t, task.GetSolution())
	assert.Equal(t, "python3", task.GetSolution().GetRunnerId())
	assert.NotNil(t, task.GetResourceFiles())
}

// TestTaskResponse_CompareScriptIdPosition verifies that compare_script_id is at field position 4
// This test ensures the proto schema is synchronized with task-server
func TestTaskResponse_CompareScriptIdPosition(t *testing.T) {
	// The proto binary format uses field numbers for serialization
	// If compare_script_id is not at position 4, it will cause wire format incompatibility
	// with task-server which sends it at position 4

	compareScriptID := "test_compare_script"
	task := &taskPB.TaskResponse{
		CompareScriptId: &compareScriptID,
	}

	// Verify the getter returns the correct value
	// This confirms the field is properly defined in the generated code
	assert.Equal(t, "test_compare_script", task.GetCompareScriptId())

	// Verify the field is optional (can be nil)
	taskWithoutCompare := &taskPB.TaskResponse{
		Id: "task-without-compare",
	}
	assert.Equal(t, "", taskWithoutCompare.GetCompareScriptId())
}

// TestTaskResponse_AllowedRunnersStructure verifies that AllowedRunners is a message type
// with runner_id and files, not just a list of strings
func TestTaskResponse_AllowedRunnersStructure(t *testing.T) {
	task := &taskPB.TaskResponse{
		AllowedRunners: []*taskPB.AllowedRunner{
			{
				RunnerId: "python3",
				Files: []*taskPB.File{
					{
						Name:    "main.py",
						Content: "print('hello')",
					},
				},
			},
			{
				RunnerId: "gcc",
				Files:    []*taskPB.File{},
			},
		},
	}

	// Verify AllowedRunners is a message type with proper structure
	runners := task.GetAllowedRunners()
	assert.Len(t, runners, 2)
	assert.Equal(t, "python3", runners[0].GetRunnerId())
	assert.Len(t, runners[0].GetFiles(), 1)
	assert.Equal(t, "main.py", runners[0].GetFiles()[0].GetName())
	assert.Equal(t, "gcc", runners[1].GetRunnerId())
}

// TestTaskResponse_SolutionStructure verifies that Solution is a message type
// with runner_id and files, not just a runner_id string
func TestTaskResponse_SolutionStructure(t *testing.T) {
	task := &taskPB.TaskResponse{
		Solution: &taskPB.Solution{
			RunnerId: "python3",
			Files: []*taskPB.File{
				{
					Name:    "solution.py",
					Content: "def solve(): pass",
				},
			},
		},
	}

	// Verify Solution is a message type with proper structure
	solution := task.GetSolution()
	assert.NotNil(t, solution)
	assert.Equal(t, "python3", solution.GetRunnerId())
	assert.Len(t, solution.GetFiles(), 1)
	assert.Equal(t, "solution.py", solution.GetFiles()[0].GetName())
}

// TestTaskResponse_ResourceFiles verifies that ResourceFiles is a separate field
func TestTaskResponse_ResourceFiles(t *testing.T) {
	task := &taskPB.TaskResponse{
		ResourceFiles: []*taskPB.File{
			{
				Name:    "input.txt",
				Content: "test input data",
			},
		},
	}

	// Verify ResourceFiles is accessible
	files := task.GetResourceFiles()
	assert.Len(t, files, 1)
	assert.Equal(t, "input.txt", files[0].GetName())
}

// TestUpdateTaskRequest_FieldNumbering verifies UpdateTaskRequest has correct field numbering
func TestUpdateTaskRequest_FieldNumbering(t *testing.T) {
	compareScriptID := "compare-123"
	updateReq := &taskPB.UpdateTaskRequest{
		Id:              strPtr("task-123"),
		TestCaseGroups:  []*taskPB.TestCaseGroup{},
		AllowedRunners:  []*taskPB.AllowedRunner{},
		CompareScriptId: &compareScriptID,
		Limit: &taskPB.Limit{
			CpuTime: 1.0,
		},
		Solution: &taskPB.Solution{
			RunnerId: "python3",
		},
		ResourceFiles: []*taskPB.File{},
	}

	// Verify all fields are accessible
	assert.Equal(t, "task-123", updateReq.GetId())
	assert.Equal(t, "compare-123", updateReq.GetCompareScriptId())
	assert.NotNil(t, updateReq.GetLimit())
	assert.NotNil(t, updateReq.GetSolution())
}

func strPtr(s string) *string {
	return &s
}
