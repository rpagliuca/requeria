-- Requisito obsoleto: continua no histórico, com quando e por quê, mas deixa de limitar os pais e
-- de pesar entre os irmãos. Sem obsoleto_em, está vivo.
ALTER TABLE requisitos ADD COLUMN obsoleto_em TEXT;
ALTER TABLE requisitos ADD COLUMN motivo_obsoleto TEXT
    CHECK (motivo_obsoleto IS NULL OR length(trim(motivo_obsoleto)) > 0);
