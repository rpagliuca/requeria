-- Quão exata é a hora marcada: hora (sensível ao horário), turno ou dia (avisados em lote, sem
-- alarme). Os lembretes de antes eram todos de hora marcada.
ALTER TABLE lembretes ADD COLUMN precisao TEXT NOT NULL DEFAULT 'hora' CHECK (precisao IN ('hora', 'turno', 'dia'));
