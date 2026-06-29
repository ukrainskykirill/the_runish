-- Онбординг-анкета бегового клуба: состояние диалога в боте + ответы.
-- Одна запись на пользователя. Создаётся со статусом 'pending' ТОЛЬКО при
-- первой регистрации (INSERT нового юзера) — так анкета достаётся только новым.

CREATE TABLE survey_responses (
    user_id      BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    status       TEXT NOT NULL DEFAULT 'pending',   -- 'pending' | 'in_progress' | 'completed'
    branch       TEXT,                              -- 'novice' | 'casual' | 'regular'
    step         TEXT,                              -- ключ текущего шага анкеты
    answers      JSONB NOT NULL DEFAULT '{}',       -- map step_key -> string | []int (мультивыбор)
    msg_id       BIGINT,                            -- message_id текущего вопроса (для редактирования клавиатуры)
    started_at   TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
