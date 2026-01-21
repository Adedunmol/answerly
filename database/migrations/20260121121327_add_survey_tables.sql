-- +goose Up
-- +goose StatementBegin
-- Survey table: Contains survey metadata
CREATE TABLE surveys (
                         id BIGSERIAL PRIMARY KEY,
                         title VARCHAR(255) NOT NULL,
                         description TEXT,
                         category VARCHAR(100) NOT NULL,
                         estimated_time_minutes INT NOT NULL,
                         reward DECIMAL(10, 2) NOT NULL,
                         eligibility JSONB, -- Store eligibility criteria as JSON
                         status VARCHAR(50) DEFAULT 'active', -- active, paused, closed
                         created_by BIGINT NOT NULL,
                         created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
                         updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Questions table: Contains survey questions
CREATE TABLE survey_questions (
                                  id BIGSERIAL PRIMARY KEY,
                                  survey_id BIGINT NOT NULL REFERENCES surveys(id) ON DELETE CASCADE,
                                  question_text TEXT NOT NULL,
                                  question_type VARCHAR(50) NOT NULL, -- multiple_choice, single_choice, text, rating, etc.
                                  is_required BOOLEAN DEFAULT true,
                                  order_index INT NOT NULL,
                                  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Question options table: Contains options for multiple choice questions
CREATE TABLE question_options (
                                  id BIGSERIAL PRIMARY KEY,
                                  question_id BIGINT NOT NULL REFERENCES survey_questions(id) ON DELETE CASCADE,
                                  option_text TEXT NOT NULL,
                                  order_index INT NOT NULL,
                                  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- User survey responses table: Tracks user participation and progress
CREATE TABLE user_survey_responses (
                                       id BIGSERIAL PRIMARY KEY,
                                       user_id BIGINT NOT NULL,
                                       survey_id BIGINT NOT NULL REFERENCES surveys(id) ON DELETE CASCADE,
                                       status VARCHAR(50) DEFAULT 'in_progress', -- in_progress, completed, abandoned
                                       percentage_completed DECIMAL(5, 2) DEFAULT 0.00,
                                       started_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
                                       completed_at TIMESTAMP,
                                       updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
                                       UNIQUE(user_id, survey_id)
);

-- Answer responses table: Stores individual question answers
CREATE TABLE answer_responses (
                                  id BIGSERIAL PRIMARY KEY,
                                  user_survey_response_id BIGINT NOT NULL REFERENCES user_survey_responses(id) ON DELETE CASCADE,
                                  question_id BIGINT NOT NULL REFERENCES survey_questions(id) ON DELETE CASCADE,
                                  answer_text TEXT, -- For text responses
                                  selected_option_ids BIGINT[], -- For multiple/single choice (array of option IDs)
                                  answered_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
                                  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
                                  UNIQUE(user_survey_response_id, question_id)
);

-- Indexes for performance
CREATE INDEX idx_surveys_category ON surveys(category);
CREATE INDEX idx_surveys_status ON surveys(status);
CREATE INDEX idx_survey_questions_survey_id ON survey_questions(survey_id);
CREATE INDEX idx_question_options_question_id ON question_options(question_id);
CREATE INDEX idx_user_survey_responses_user_id ON user_survey_responses(user_id);
CREATE INDEX idx_user_survey_responses_survey_id ON user_survey_responses(survey_id);
CREATE INDEX idx_answer_responses_user_survey_response_id ON answer_responses(user_survey_response_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_answer_responses_user_survey_response_id;
DROP INDEX IF EXISTS idx_user_survey_responses_survey_id;
DROP INDEX IF EXISTS idx_user_survey_responses_user_id;
DROP INDEX IF EXISTS idx_question_options_question_id;
DROP INDEX IF EXISTS idx_survey_questions_survey_id;
DROP INDEX IF EXISTS idx_surveys_status;
DROP INDEX IF EXISTS idx_surveys_category;

-- Drop tables in reverse order (respecting foreign key constraints)
DROP TABLE IF EXISTS answer_responses;
DROP TABLE IF EXISTS user_survey_responses;
DROP TABLE IF EXISTS question_options;
DROP TABLE IF EXISTS survey_questions;
DROP TABLE IF EXISTS surveys;
-- +goose StatementEnd
