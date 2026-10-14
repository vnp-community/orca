package wscompat

import (
	"context"
	"encoding/json"
	"errors"
	"sort"

	"github.com/stablyai/orca-go/common/secretscan"
	taskv1 "github.com/stablyai/orca-go/proto/gen/go/orca/task/v1"
)

// maxWritebackTasks is how many task ids one ListExecutionRecords call takes.
const maxWritebackTasks = 200

// TaskWritebackView is what one task's agent said it wrote back to the design knowledge. It is the agent's own report: the
// documents it says it changed and the ChangeSet proposal it says it made. Nothing here has been applied by anyone.
type TaskWritebackView struct {
	TaskID     string   `json:"taskId"`
	Title      string   `json:"title"`
	Attempt    int32    `json:"attempt,omitempty"`
	Documents  []string `json:"documents"`
	ProposalID string   `json:"proposalId,omitempty"`
	ProposalOK bool     `json:"proposalOk"`
	Touched    []string `json:"touched"`
}

// WritebackTotals counts across the Plan; ProposalsNotOK are proposals the agent itself reported as not ok.
type WritebackTotals struct {
	Tasks          int `json:"tasks"`
	WithWriteback  int `json:"withWriteback"`
	Proposals      int `json:"proposals"`
	ProposalsNotOK int `json:"proposalsNotOk"`
	Documents      int `json:"documents"`
	Touched        int `json:"touched"`
}

type WritebackSummaryView struct {
	Tasks   []TaskWritebackView `json:"tasks"`
	Summary WritebackTotals     `json:"summary"`
}

// WritebacksFor gathers the write-back reports of every working task under a Plan with one subtree read and one records
// read. Task-service checks read access on the tasks; a task the caller may not read makes the whole read fail, as for
// every other task view.
func WritebacksFor(ctx context.Context, client taskv1.TaskServiceClient, planTaskID string) (*WritebackSummaryView, error) {
	sub, err := client.GetSubtree(ctx, &taskv1.GetSubtreeRequest{RootId: planTaskID})
	if err != nil {
		return nil, executionChannelError(err)
	}
	titles := map[string]string{}
	var ids []string
	for _, t := range sub.GetTasks() {
		switch t.GetTaskType() {
		case "plan", "phase", "epic":
			continue
		}
		titles[t.GetId()] = t.GetTitle()
		ids = append(ids, t.GetId())
	}
	out := &WritebackSummaryView{Tasks: []TaskWritebackView{}, Summary: WritebackTotals{Tasks: len(ids)}}
	if len(ids) == 0 {
		return out, nil
	}
	if len(ids) > maxWritebackTasks {
		return nil, errors.New("TASK_INVALID_ARGUMENT: the plan has more tasks than one read covers")
	}
	sort.Strings(ids)
	recs, err := client.ListExecutionRecords(ctx, &taskv1.ListExecutionRecordsRequest{TaskIds: ids, LatestOnly: true})
	if err != nil {
		return nil, executionChannelError(err)
	}
	for _, rec := range recs.GetRecords() {
		wb, ok := writebackOf(rec)
		if !ok {
			continue
		}
		wb.TaskID, wb.Title, wb.Attempt = rec.GetTaskId(), titles[rec.GetTaskId()], rec.GetAttempt()
		out.Tasks = append(out.Tasks, wb)
		out.Summary.WithWriteback++
		out.Summary.Documents += len(wb.Documents)
		out.Summary.Touched += len(wb.Touched)
		if wb.ProposalID != "" {
			out.Summary.Proposals++
			if !wb.ProposalOK {
				out.Summary.ProposalsNotOK++
			}
		}
	}
	sort.Slice(out.Tasks, func(i, j int) bool { return out.Tasks[i].TaskID < out.Tasks[j].TaskID })
	return out, nil
}

// writebackOf reads the writeback block of one stored result. A record without the block, or whose block is empty, reports
// nothing and is left out.
func writebackOf(rec *taskv1.ExecutionRecord) (TaskWritebackView, bool) {
	var result struct {
		Writeback *struct {
			Documents  []string `json:"documents"`
			ProposalID string   `json:"proposal_id"`
			ProposalOK bool     `json:"proposal_ok"`
			Touched    []string `json:"touched"`
		} `json:"writeback"`
	}
	if rec.GetResultJson() == "" || json.Unmarshal([]byte(rec.GetResultJson()), &result) != nil || result.Writeback == nil {
		return TaskWritebackView{}, false
	}
	w := result.Writeback
	if len(w.Documents) == 0 && w.ProposalID == "" && len(w.Touched) == 0 {
		return TaskWritebackView{}, false
	}
	id, _ := secretscan.Redact(w.ProposalID)
	return TaskWritebackView{Documents: redactedList(w.Documents), ProposalID: id, ProposalOK: w.ProposalOK, Touched: redactedList(w.Touched)}, true
}
