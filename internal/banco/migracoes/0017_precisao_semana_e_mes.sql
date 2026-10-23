-- Precisão também semana e mês (avisados em lote, sem alarme, como turno e dia). O SQLite não
-- altera um CHECK: a tabela é recriada com as mesmas colunas, e nada referencia lembretes.

CREATE TABLE lembretes_novo (
    id            INTEGER PRIMARY KEY,
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
    precisao      TEXT NOT NULL DEFAULT 'hora' CHECK (precisao IN ('hora', 'turno', 'dia', 'semana', 'mes')),
    CHECK ((status = 'confirmado') = (confirmado_em IS NOT NULL))
);

INSERT INTO lembretes_novo (id, texto, quando, quando_utc, organizacao, tarefa_id, requisito_id, status, autor,
    disparado_em, atrasado, adiamentos, confirmado_em, criado_em, atualizado_em, precisao)
SELECT id, texto, quando, quando_utc, organizacao, tarefa_id, requisito_id, status, autor,
    disparado_em, atrasado, adiamentos, confirmado_em, criado_em, atualizado_em, precisao
FROM lembretes;

DROP TABLE lembretes;
ALTER TABLE lembretes_novo RENAME TO lembretes;
CREATE INDEX lembretes_pendentes ON lembretes (quando_utc) WHERE status IN ('agendado', 'adiado', 'disparado');
