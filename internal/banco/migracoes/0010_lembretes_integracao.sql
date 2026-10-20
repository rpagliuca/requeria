-- Lembretes com hora marcada, espera de resposta das sessões, idempotência da captura e cursores
-- dos ganchos de integração.

CREATE TABLE lembretes (
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
    CHECK ((status = 'confirmado') = (confirmado_em IS NOT NULL))
);
CREATE INDEX lembretes_pendentes ON lembretes (quando_utc) WHERE status IN ('agendado', 'adiado', 'disparado');

ALTER TABLE sessoes ADD COLUMN aguardando TEXT;
ALTER TABLE sessoes ADD COLUMN aguardando_desde TEXT;

-- Uma captura repetida (mesma chave) devolve a resposta guardada.
CREATE TABLE capturas (
    chave     TEXT PRIMARY KEY,
    status    INTEGER NOT NULL,
    resposta  TEXT NOT NULL,
    criado    TEXT,
    criada_em TEXT NOT NULL
);

-- Último evento entregue por cada gancho: depois de reiniciar, a entrega continua de onde parou.
CREATE TABLE cursores_de_integracao (
    gancho TEXT PRIMARY KEY,
    evento INTEGER NOT NULL
);
