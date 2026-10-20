-- Um componente pode pertencer a várias organizações (a toggle do topo lista só os delas).
CREATE TABLE componente_organizacao (
    componente_id INTEGER NOT NULL REFERENCES componentes (id),
    organizacao   TEXT NOT NULL CHECK (length(trim(organizacao)) > 0),
    PRIMARY KEY (componente_id, organizacao)
);
INSERT INTO componente_organizacao (componente_id, organizacao) SELECT id, organizacao FROM componentes;
ALTER TABLE componentes DROP COLUMN organizacao;
