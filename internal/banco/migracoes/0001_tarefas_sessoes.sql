-- Tarefas, Sessões de Trabalho e log de eventos.
-- As invariantes ficam no próprio banco, além da API: nenhuma escrita consegue violá-las.

CREATE TABLE tarefas (
    id            INTEGER PRIMARY KEY,
    titulo        TEXT NOT NULL CHECK (length(trim(titulo)) > 0),
    descricao     TEXT NOT NULL DEFAULT '',
    organizacao   TEXT NOT NULL CHECK (length(trim(organizacao)) > 0),
    status        TEXT NOT NULL DEFAULT 'a_fazer'
                  CHECK (status IN ('a_fazer', 'em_andamento', 'pausada', 'bloqueada', 'em_validacao', 'concluida')),
    resumo        TEXT NOT NULL DEFAULT '',
    criada_em     TEXT NOT NULL,
    atualizada_em TEXT NOT NULL
);

CREATE TABLE sessoes (
    id                  INTEGER PRIMARY KEY,
    status              TEXT NOT NULL DEFAULT 'ativa' CHECK (status IN ('ativa', 'encerrada', 'substituida')),
    agente              TEXT NOT NULL CHECK (length(trim(agente)) > 0),
    tipo_agente         TEXT NOT NULL CHECK (tipo_agente IN ('ia', 'humano')),
    chat                TEXT,
    servidor            TEXT NOT NULL CHECK (length(trim(servidor)) > 0),
    -- Cada sessão é substituída no máximo uma vez.
    anterior_id         INTEGER UNIQUE REFERENCES sessoes (id),
    motivo_encerramento TEXT CHECK (motivo_encerramento IN ('fim_do_trabalho', 'troca_de_agente')),
    journal             TEXT,
    artefatos           TEXT NOT NULL DEFAULT '[]',
    token_hash          TEXT NOT NULL UNIQUE,
    iniciada_em         TEXT NOT NULL,
    encerrada_em        TEXT,
    CHECK ((status = 'ativa') = (encerrada_em IS NULL)),
    CHECK ((status = 'ativa') = (motivo_encerramento IS NULL))
);

-- O agente de uma sessão nunca muda: trocar de agente é encerrar e repassar.
CREATE TRIGGER sessoes_agente_imutavel
BEFORE UPDATE OF agente, tipo_agente ON sessoes
WHEN NEW.agente IS NOT OLD.agente OR NEW.tipo_agente IS NOT OLD.tipo_agente
BEGIN
    SELECT RAISE(ABORT, 'o agente de uma sessão é imutável');
END;

CREATE TRIGGER sessoes_encerradas_imutaveis
BEFORE UPDATE ON sessoes
WHEN OLD.status <> 'ativa'
BEGIN
    SELECT RAISE(ABORT, 'uma sessão encerrada não pode ser alterada');
END;

CREATE TABLE sessao_tarefa (
    sessao_id INTEGER NOT NULL REFERENCES sessoes (id),
    tarefa_id INTEGER NOT NULL REFERENCES tarefas (id),
    ativa     INTEGER NOT NULL DEFAULT 1 CHECK (ativa IN (0, 1)),
    PRIMARY KEY (sessao_id, tarefa_id)
);

-- Cada tarefa tem no máximo uma sessão de trabalho ativa.
CREATE UNIQUE INDEX uma_sessao_ativa_por_tarefa ON sessao_tarefa (tarefa_id) WHERE ativa = 1;

CREATE TABLE eventos (
    id       INTEGER PRIMARY KEY,
    momento  TEXT NOT NULL,
    autor    TEXT NOT NULL,
    entidade TEXT NOT NULL,
    tipo     TEXT NOT NULL,
    dados    TEXT NOT NULL DEFAULT '{}'
);

CREATE INDEX eventos_por_entidade ON eventos (entidade);

CREATE TRIGGER eventos_sem_update
BEFORE UPDATE ON eventos
BEGIN
    SELECT RAISE(ABORT, 'o log de eventos é append-only');
END;

CREATE TRIGGER eventos_sem_delete
BEFORE DELETE ON eventos
BEGIN
    SELECT RAISE(ABORT, 'o log de eventos é append-only');
END;
