-- Lembretes podem ser apagados de vez (privacidade). AUTOINCREMENT impede que um id apagado volte
-- num lembrete novo, o que faria os eventos antigos do log parecerem dele.

CREATE TABLE lembretes_novo (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    texto         TEXT NOT NULL CHECK (length(trim(texto)) > 0),
    -- Instante com o fuso em que foi pedido (RFC 3339); quando_utc ordena e compara.
    quando        TEXT NOT NULL,
    quando_utc    TEXT NOT NULL,
    organizacao   TEXT NOT NULL CHECK (length(trim(organizacao)) > 0),
    tarefa_id     INTEGER REFERENCES tarefas (id),
    requisito_id  INTEGER REFERENCES requisitos (id),
    status        TEXT NOT NULL DEFAULT 'agendado'
                  CHECK (status IN ('agendado', 'disparado', 'adiado', 'confirmado', 'cancelado')),
    autor         TEXT NOT NULL,
    disparado_em  TEXT,
    atrasado      INTEGER NOT NULL DEFAULT 0 CHECK (atrasado IN (0, 1)),
    adiamentos    INTEGER NOT NULL DEFAULT 0,
    confirmado_em TEXT,
    criado_em     TEXT NOT NULL,
    atualizado_em TEXT NOT NULL,
    CHECK ((status = 'confirmado') = (confirmado_em IS NOT NULL))
);
INSERT INTO lembretes_novo SELECT * FROM lembretes;
DROP TABLE lembretes;
ALTER TABLE lembretes_novo RENAME TO lembretes;
CREATE INDEX lembretes_pendentes ON lembretes (quando_utc) WHERE status IN ('agendado', 'adiado', 'disparado');

-- O maior id já usado, mesmo que o lembrete dele já não exista (vem do log de eventos).
DELETE FROM sqlite_sequence WHERE name = 'lembretes';
INSERT INTO sqlite_sequence (name, seq)
SELECT 'lembretes', max(
    (SELECT coalesce(max(id), 0) FROM lembretes),
    (SELECT coalesce(max(CAST(substr(entidade, 3) AS INTEGER)), 0) FROM eventos WHERE entidade LIKE 'L-%'));
