ALTER TABLE proposal_periods
    RENAME TO suggestion_periods;

ALTER SEQUENCE proposal_periods_id_seq
    RENAME TO suggestion_periods_id_seq;

ALTER TABLE suggestion_periods
    RENAME CONSTRAINT proposal_periods_pkey TO suggestion_periods_pkey;

ALTER TABLE suggestion_periods
    RENAME CONSTRAINT proposal_periods_closes_after_create TO suggestion_periods_closes_after_create;

ALTER TABLE suggestion_periods
    RENAME CONSTRAINT proposal_periods_no_overlap TO suggestion_periods_no_overlap;

ALTER TABLE study_proposals
    RENAME TO study_suggestions;

ALTER SEQUENCE study_proposals_id_seq
    RENAME TO study_suggestions_id_seq;

ALTER TABLE study_suggestions
    RENAME CONSTRAINT study_proposals_pkey TO study_suggestions_pkey;

ALTER TABLE study_suggestions
    RENAME CONSTRAINT study_proposals_period_id_fkey TO study_suggestions_period_id_fkey;

ALTER TABLE study_suggestions
    RENAME CONSTRAINT study_proposals_message_id_non_empty TO study_suggestions_message_id_non_empty;

ALTER TABLE study_suggestions
    RENAME CONSTRAINT study_proposals_channel_id_non_empty TO study_suggestions_channel_id_non_empty;

ALTER TABLE study_proposal_votes
    RENAME TO study_suggestion_votes;

ALTER TABLE study_suggestion_votes
    RENAME COLUMN proposal_id TO suggestion_id;

ALTER TABLE study_suggestion_votes
    RENAME CONSTRAINT study_proposal_votes_pkey TO study_suggestion_votes_pkey;

ALTER TABLE study_suggestion_votes
    RENAME CONSTRAINT study_proposal_votes_proposal_id_fkey TO study_suggestion_votes_suggestion_id_fkey;
