package main

type reviewConfig struct {
	// AllowSelfApprove is the tag which indicate if the author
	// can appove his/her own pull-request.
	AllowSelfApprove bool `json:"allow_self_approve"`

	// NumberOfApprovers is the min number of approvers who commented
	// /approve at same time to approve a single module
	NumberOfApprovers int `json:"number_of_approvers"`

	// TotalNumberOfApprovers is the min number of approvers who commented
	// /approve at same time to add approved label
	TotalNumberOfApprovers int `json:"total_number_of_approvers"`

	// TotalNumberOfReviewers is the min number of reviewers who commented
	// /lgtm at same time to add lgtm label
	TotalNumberOfReviewers int `json:"total_number_of_reviewers"`
}

func (r reviewConfig) validate() error {
	return nil
}

func (r *reviewConfig) setDefault() {
	if r == nil {
		return
	}

	if r.NumberOfApprovers <= 0 {
		r.NumberOfApprovers = 1
	}

	if r.TotalNumberOfApprovers <= 0 {
		r.TotalNumberOfApprovers = 2
	}

	if r.TotalNumberOfReviewers == 0 {
		r.TotalNumberOfReviewers = 1
	}
}
