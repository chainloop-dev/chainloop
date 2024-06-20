-- Modify "cas_backends" table
ALTER TABLE "cas_backends" ADD COLUMN "max_blob_size_bytes" bigint NULL;

-- Update the max_blob_size_bytes column for both inline and non-inline cas backends
UPDATE public.cas_backends SET max_blob_size_bytes = 104857600 WHERE provider != 'INLINE';
UPDATE public.cas_backends SET max_blob_size_bytes = 512000 WHERE provider = 'INLINE';

-- Enable the not null constraint
ALTER TABLE "cas_backends" ALTER COLUMN "max_blob_size_bytes" SET NOT NULL;
