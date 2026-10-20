-- Origens, Requisitos, a cadeia de derivação entre eles e o vínculo Tarefa ↔ Requisito.

CREATE TABLE origens (
    id          INTEGER PRIMARY KEY,
    nome        TEXT NOT NULL UNIQUE CHECK (length(trim(nome)) > 0),
    tipo        TEXT NOT NULL CHECK (tipo IN ('pessoa', 'documento')),
    descricao   TEXT NOT NULL DEFAULT '',
    criada_em   TEXT NOT NULL
);

CREATE TABLE requisitos (
    id            INTEGER PRIMARY KEY,
    titulo        TEXT NOT NULL CHECK (length(trim(titulo)) > 0),
    descricao     TEXT NOT NULL DEFAULT '',
    organizacao   TEXT NOT NULL CHECK (length(trim(organizacao)) > 0),
    origem_id     INTEGER REFERENCES origens (id),
    criado_em     TEXT NOT NULL,
    atualizado_em TEXT NOT NULL
);

-- Grafo: um requisito deriva de zero ou mais requisitos. Ciclos são recusados pela API (o SQLite
-- não aceita consultas recursivas em gatilhos); laço direto, pelo banco.
CREATE TABLE derivacoes (
    requisito_id INTEGER NOT NULL REFERENCES requisitos (id),
    pai_id       INTEGER NOT NULL REFERENCES requisitos (id),
    PRIMARY KEY (requisito_id, pai_id),
    CHECK (requisito_id <> pai_id)
);
CREATE INDEX derivacoes_por_pai ON derivacoes (pai_id);

CREATE TABLE tarefa_requisito (
    tarefa_id    INTEGER NOT NULL REFERENCES tarefas (id),
    requisito_id INTEGER NOT NULL REFERENCES requisitos (id),
    PRIMARY KEY (tarefa_id, requisito_id)
);
CREATE INDEX tarefa_requisito_por_requisito ON tarefa_requisito (requisito_id);
