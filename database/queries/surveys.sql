-- name: CreateSurvey :one
INSERT INTO surveys (
    title,
    description,
    category,
    estimated_time_minutes,
    reward,
    eligibility,
    created_by
) VALUES (
             $1, $2, $3, $4, $5, $6, $7
         ) RETURNING *;

-- name: GetSurvey :one
SELECT * FROM surveys
WHERE id = $1;

-- name: GetSurveyWithQuestions :many
SELECT
    s.*,
    sq.id as question_id,
    sq.question_text,
    sq.question_type,
    sq.is_required,
    sq.order_index as question_order
FROM surveys s
         LEFT JOIN survey_questions sq ON s.id = sq.survey_id
WHERE s.id = $1
ORDER BY sq.order_index;

-- name: ListSurveys :many
SELECT * FROM surveys
WHERE
    (sqlc.narg('category')::VARCHAR IS NULL OR category = sqlc.narg('category'))
  AND (sqlc.narg('status')::VARCHAR IS NULL OR status = sqlc.narg('status'))
ORDER BY created_at DESC
    LIMIT $1 OFFSET $2;

-- name: UpdateSurvey :one
UPDATE surveys
SET
    title = COALESCE(sqlc.narg('title'), title),
    description = COALESCE(sqlc.narg('description'), description),
    category = COALESCE(sqlc.narg('category'), category),
    estimated_time_minutes = COALESCE(sqlc.narg('estimated_time_minutes'), estimated_time_minutes),
    reward = COALESCE(sqlc.narg('reward'), reward),
    eligibility = COALESCE(sqlc.narg('eligibility'), eligibility),
    status = COALESCE(sqlc.narg('status'), status),
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1
    RETURNING *;

-- name: DeleteSurvey :exec
DELETE FROM surveys
WHERE id = $1;

-- ==================== Question Management ====================

-- name: CreateQuestion :one
INSERT INTO survey_questions (
    survey_id,
    question_text,
    question_type,
    is_required,
    order_index
) VALUES (
             $1, $2, $3, $4, $5
         ) RETURNING *;

-- name: GetQuestion :one
SELECT * FROM survey_questions
WHERE id = $1;

-- name: GetQuestionsBySurveyID :many
SELECT * FROM survey_questions
WHERE survey_id = $1
ORDER BY order_index;

-- name: UpdateQuestion :one
UPDATE survey_questions
SET
    question_text = COALESCE(sqlc.narg('question_text'), question_text),
    question_type = COALESCE(sqlc.narg('question_type'), question_type),
    is_required = COALESCE(sqlc.narg('is_required'), is_required),
    order_index = COALESCE(sqlc.narg('order_index'), order_index)
WHERE id = $1
    RETURNING *;

-- name: DeleteQuestion :exec
DELETE FROM survey_questions
WHERE id = $1;

-- ==================== Question Options Management ====================

-- name: CreateQuestionOption :one
INSERT INTO question_options (
    question_id,
    option_text,
    order_index
) VALUES (
             $1, $2, $3
         ) RETURNING *;

-- name: GetOptionsByQuestionID :many
SELECT * FROM question_options
WHERE question_id = $1
ORDER BY order_index;

-- name: GetOptionsByQuestionIDs :many
SELECT * FROM question_options
WHERE question_id = ANY($1::bigint[])
ORDER BY question_id, order_index;

-- name: UpdateQuestionOption :one
UPDATE question_options
SET
    option_text = COALESCE(sqlc.narg('option_text'), option_text),
    order_index = COALESCE(sqlc.narg('order_index'), order_index)
WHERE id = $1
    RETURNING *;

-- name: DeleteQuestionOption :exec
DELETE FROM question_options
WHERE id = $1;

-- ==================== User Survey Response Management ====================

-- name: StartSurvey :one
INSERT INTO user_survey_responses (
    user_id,
    survey_id,
    status,
    percentage_completed
) VALUES (
             $1, $2, 'in_progress', 0.00
         ) RETURNING *;

-- name: GetUserSurveyResponse :one
SELECT * FROM user_survey_responses
WHERE user_id = $1 AND survey_id = $2;

-- name: GetUserSurveyResponseByID :one
SELECT * FROM user_survey_responses
WHERE id = $1;

-- name: GetUserSurveyProgress :one
SELECT
    usr.*,
    COUNT(DISTINCT sq.id) as total_questions,
    COUNT(DISTINCT ar.question_id) as answered_questions,
    CASE
        WHEN COUNT(DISTINCT sq.id) > 0
            THEN ROUND((COUNT(DISTINCT ar.question_id)::DECIMAL / COUNT(DISTINCT sq.id)::DECIMAL) * 100, 2)
        ELSE 0.00
        END as calculated_percentage
FROM user_survey_responses usr
         INNER JOIN surveys s ON usr.survey_id = s.id
         LEFT JOIN survey_questions sq ON s.id = sq.survey_id
         LEFT JOIN answer_responses ar ON usr.id = ar.user_survey_response_id AND sq.id = ar.question_id
WHERE usr.user_id = $1 AND usr.survey_id = $2
GROUP BY usr.id;

-- name: ListUserSurveys :many
SELECT
    s.*,
    usr.id as user_survey_response_id,
    usr.status as response_status,
    usr.percentage_completed,
    usr.started_at,
    usr.completed_at,
    usr.updated_at as response_updated_at
FROM user_survey_responses usr
         INNER JOIN surveys s ON usr.survey_id = s.id
WHERE usr.user_id = $1
  AND (sqlc.narg('status')::VARCHAR IS NULL OR usr.status = sqlc.narg('status'))
ORDER BY usr.updated_at DESC;

-- name: UpdateSurveyProgress :one
UPDATE user_survey_responses
SET
    percentage_completed = $2,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1
    RETURNING *;

-- name: CompleteSurvey :one
UPDATE user_survey_responses
SET
    status = 'completed',
    percentage_completed = 100.00,
    completed_at = CURRENT_TIMESTAMP,
    updated_at = CURRENT_TIMESTAMP
WHERE user_id = $1 AND survey_id = $2
    RETURNING *;

-- name: UpdateUserSurveyResponseStatus :one
UPDATE user_survey_responses
SET
    status = $2,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1
    RETURNING *;

-- ==================== Answer Management ====================

-- name: SaveAnswer :one
INSERT INTO answer_responses (
    user_survey_response_id,
    question_id,
    answer_text,
    selected_option_ids
) VALUES (
             $1, $2, $3, $4
         )
    ON CONFLICT (user_survey_response_id, question_id)
DO UPDATE SET
    answer_text = EXCLUDED.answer_text,
           selected_option_ids = EXCLUDED.selected_option_ids,
           updated_at = CURRENT_TIMESTAMP
           RETURNING *;

-- name: GetAnswer :one
SELECT * FROM answer_responses
WHERE user_survey_response_id = $1 AND question_id = $2;

-- name: GetAnswersByUserSurveyResponse :many
SELECT * FROM answer_responses
WHERE user_survey_response_id = $1
ORDER BY answered_at;

-- name: GetAnswersWithQuestions :many
SELECT
    ar.*,
    sq.question_text,
    sq.question_type,
    sq.order_index
FROM answer_responses ar
         INNER JOIN survey_questions sq ON ar.question_id = sq.id
WHERE ar.user_survey_response_id = $1
ORDER BY sq.order_index;

-- name: UpdateAnswer :one
UPDATE answer_responses
SET
    answer_text = COALESCE(sqlc.narg('answer_text'), answer_text),
    selected_option_ids = COALESCE(sqlc.narg('selected_option_ids'), selected_option_ids),
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1
    RETURNING *;

-- name: DeleteAnswer :exec
DELETE FROM answer_responses
WHERE id = $1;

-- name: DeleteAnswersByUserSurveyResponse :exec
DELETE FROM answer_responses
WHERE user_survey_response_id = $1;

-- name: CountAnsweredQuestions :one
SELECT COUNT(*) FROM answer_responses
WHERE user_survey_response_id = $1;

-- ==================== Complex Queries for Survey Details ====================

-- name: GetSurveyDetailWithQuestionsAndOptions :many
SELECT
    s.id as survey_id,
    s.title as survey_title,
    s.description as survey_description,
    s.category as survey_category,
    s.estimated_time_minutes,
    s.reward,
    s.eligibility,
    s.status as survey_status,
    s.created_by,
    s.created_at as survey_created_at,
    sq.id as question_id,
    sq.question_text,
    sq.question_type,
    sq.is_required,
    sq.order_index as question_order,
    qo.id as option_id,
    qo.option_text,
    qo.order_index as option_order
FROM surveys s
         LEFT JOIN survey_questions sq ON s.id = sq.survey_id
         LEFT JOIN question_options qo ON sq.id = qo.question_id
WHERE s.id = $1
ORDER BY sq.order_index, qo.order_index;

-- name: GetUserSurveyWithAnswers :many
SELECT
    usr.*,
    s.title as survey_title,
    s.category,
    s.estimated_time_minutes,
    s.reward,
    ar.id as answer_id,
    ar.question_id,
    ar.answer_text,
    ar.selected_option_ids,
    sq.question_text,
    sq.question_type
FROM user_survey_responses usr
         INNER JOIN surveys s ON usr.survey_id = s.id
         LEFT JOIN answer_responses ar ON usr.id = ar.user_survey_response_id
         LEFT JOIN survey_questions sq ON ar.question_id = sq.id
WHERE usr.user_id = $1 AND usr.survey_id = $2
ORDER BY sq.order_index;

-- ==================== Analytics/Stats Queries ====================

-- name: GetSurveyStats :one
SELECT
    s.id,
    s.title,
    COUNT(DISTINCT usr.user_id) as total_participants,
    COUNT(DISTINCT CASE WHEN usr.status = 'completed' THEN usr.user_id END) as completed_count,
    COUNT(DISTINCT CASE WHEN usr.status = 'in_progress' THEN usr.user_id END) as in_progress_count,
    AVG(usr.percentage_completed) as avg_completion_percentage
FROM surveys s
         LEFT JOIN user_survey_responses usr ON s.id = usr.survey_id
WHERE s.id = $1
GROUP BY s.id;

-- name: GetUserCompletedSurveysCount :one
SELECT COUNT(*) FROM user_survey_responses
WHERE user_id = $1 AND status = 'completed';

-- name: GetUserTotalRewards :one
SELECT COALESCE(SUM(s.reward), 0) as total_rewards
FROM user_survey_responses usr
         INNER JOIN surveys s ON usr.survey_id = s.id
WHERE usr.user_id = $1 AND usr.status = 'completed';