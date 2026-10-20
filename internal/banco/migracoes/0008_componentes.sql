-- Componentes: qualquer coisa reaproveitável a que requisitos e tarefas se referem (repositório,
-- módulo, lugar físico, funcionalidade…). Sem tipo: o nome e a descrição dizem o que é.
CREATE TABLE componentes (
    id          INTEGER PRIMARY KEY,
    nome        TEXT NOT NULL UNIQUE CHECK (length(trim(nome)) > 0),
    descricao   TEXT NOT NULL DEFAULT '',
    organizacao TEXT NOT NULL CHECK (length(trim(organizacao)) > 0),
    criado_em   TEXT NOT NULL
);

CREATE TABLE requisito_componente (
    requisito_id  INTEGER NOT NULL REFERENCES requisitos (id),
    componente_id INTEGER NOT NULL REFERENCES componentes (id),
    PRIMARY KEY (requisito_id, componente_id)
);
CREATE INDEX requisito_componente_por_componente ON requisito_componente (componente_id);

CREATE TABLE tarefa_componente (
    tarefa_id     INTEGER NOT NULL REFERENCES tarefas (id),
    componente_id INTEGER NOT NULL REFERENCES componentes (id),
    PRIMARY KEY (tarefa_id, componente_id)
);
CREATE INDEX tarefa_componente_por_componente ON tarefa_componente (componente_id);
