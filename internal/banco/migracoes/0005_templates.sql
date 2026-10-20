-- Templates: tarefas que sempre precisam ser executadas, sob demanda ou periódicas. A periodicidade
-- só serve para mostrar quais estão devidos; nada é instanciado automaticamente.
CREATE TABLE templates (
    id                 INTEGER PRIMARY KEY,
    titulo             TEXT NOT NULL CHECK (length(trim(titulo)) > 0),
    descricao          TEXT NOT NULL DEFAULT '',
    organizacao        TEXT NOT NULL CHECK (length(trim(organizacao)) > 0),
    periodicidade_dias INTEGER CHECK (periodicidade_dias IS NULL OR periodicidade_dias > 0),
    criado_em          TEXT NOT NULL,
    atualizado_em      TEXT NOT NULL
);

CREATE TABLE template_requisito (
    template_id  INTEGER NOT NULL REFERENCES templates (id),
    requisito_id INTEGER NOT NULL REFERENCES requisitos (id),
    PRIMARY KEY (template_id, requisito_id)
);

ALTER TABLE tarefas ADD COLUMN template_id INTEGER REFERENCES templates (id);
CREATE INDEX tarefas_por_template ON tarefas (template_id);
