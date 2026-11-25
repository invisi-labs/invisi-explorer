CREATE TABLE IF NOT EXISTS chain_checkpoints(id TEXT PRIMARY KEY, block_number TEXT NOT NULL, block_hash TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS operations(id TEXT PRIMARY KEY, chain_id TEXT NOT NULL, protocol_address TEXT NOT NULL, block_number TEXT NOT NULL, block_hash TEXT NOT NULL, tx_hash TEXT NOT NULL, operation_index TEXT NOT NULL, canonical INTEGER NOT NULL, payload TEXT NOT NULL);
CREATE INDEX IF NOT EXISTS operations_chain ON operations(chain_id,protocol_address,block_number,id);
CREATE INDEX IF NOT EXISTS operations_tx ON operations(tx_hash,canonical);
CREATE TABLE IF NOT EXISTS checkpoints(scope TEXT NOT NULL,generation TEXT NOT NULL,sequence TEXT NOT NULL,payload TEXT NOT NULL,signature TEXT NOT NULL,published INTEGER NOT NULL,PRIMARY KEY(scope,generation,sequence));
CREATE TABLE IF NOT EXISTS public_notes(chain_id TEXT NOT NULL,protocol_address TEXT NOT NULL,commitment TEXT NOT NULL,operation_id TEXT NOT NULL,tree_id TEXT NOT NULL,leaf_index TEXT NOT NULL,token_address TEXT NOT NULL,amount TEXT NOT NULL,block_number TEXT NOT NULL,position BIGINT NOT NULL,payload TEXT NOT NULL,PRIMARY KEY(chain_id,protocol_address,commitment,operation_id));
CREATE INDEX IF NOT EXISTS public_notes_hash ON public_notes(commitment);
CREATE INDEX IF NOT EXISTS public_notes_page ON public_notes(block_number,operation_id,position);
CREATE TABLE IF NOT EXISTS public_credit_states(chain_id TEXT NOT NULL,protocol_address TEXT NOT NULL,commitment TEXT NOT NULL,operation_id TEXT NOT NULL,tree_id TEXT NOT NULL,leaf_index TEXT NOT NULL,token_address TEXT NOT NULL,amount TEXT NOT NULL,block_number TEXT NOT NULL,position BIGINT NOT NULL,payload TEXT NOT NULL,PRIMARY KEY(chain_id,protocol_address,commitment,operation_id));
CREATE INDEX IF NOT EXISTS public_credit_hash ON public_credit_states(commitment);
CREATE INDEX IF NOT EXISTS public_credit_page ON public_credit_states(block_number,operation_id,position);
CREATE TABLE IF NOT EXISTS public_nullifiers(chain_id TEXT NOT NULL,protocol_address TEXT NOT NULL,nullifier TEXT NOT NULL,kind TEXT NOT NULL,operation_id TEXT NOT NULL,block_number TEXT NOT NULL,position BIGINT NOT NULL,PRIMARY KEY(chain_id,protocol_address,nullifier,operation_id,kind));
CREATE INDEX IF NOT EXISTS public_nullifier_hash ON public_nullifiers(nullifier);
CREATE INDEX IF NOT EXISTS public_nullifier_page ON public_nullifiers(block_number,operation_id,position);
CREATE TABLE IF NOT EXISTS public_grant_ids(chain_id TEXT NOT NULL,protocol_address TEXT NOT NULL,token_address TEXT NOT NULL,payer TEXT NOT NULL,grant_id TEXT NOT NULL,operation_id TEXT NOT NULL,position BIGINT NOT NULL,block_number TEXT NOT NULL,payload TEXT NOT NULL,PRIMARY KEY(chain_id,protocol_address,token_address,payer,grant_id,operation_id));
CREATE INDEX IF NOT EXISTS public_grant_history ON public_grant_ids(chain_id,protocol_address,token_address,payer,grant_id,block_number,operation_id);
CREATE TABLE IF NOT EXISTS scopes(id TEXT PRIMARY KEY,chain_id TEXT NOT NULL,protocol_address TEXT NOT NULL,generation TEXT NOT NULL,published TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS chain_generations(id TEXT PRIMARY KEY,generation TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS sync_windows(scope TEXT PRIMARY KEY,cursor TEXT NOT NULL,target TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS staged_operations(scope TEXT NOT NULL,id TEXT PRIMARY KEY,payload TEXT NOT NULL);CREATE TABLE IF NOT EXISTS remote_recognition(operation_id TEXT PRIMARY KEY,payload TEXT NOT NULL,updated_at INTEGER NOT NULL);
CREATE TABLE IF NOT EXISTS readmodel_versions(id TEXT PRIMARY KEY,version INTEGER NOT NULL);
CREATE TABLE IF NOT EXISTS source_status(scope TEXT NOT NULL,source TEXT NOT NULL,status TEXT NOT NULL,updated_at INTEGER NOT NULL,payload TEXT NOT NULL,PRIMARY KEY(scope,source));
CREATE TABLE IF NOT EXISTS recognition_requests(operation_id TEXT PRIMARY KEY,requested_at INTEGER NOT NULL);
CREATE INDEX IF NOT EXISTS remote_recognition_age ON remote_recognition(updated_at,operation_id);
CREATE INDEX IF NOT EXISTS operations_public_time ON operations(substr('00000000000000000000'||json_extract(payload,'$.timestamp'),-20),id);
CREATE TABLE IF NOT EXISTS root_backfill(scope TEXT PRIMARY KEY,epoch TEXT NOT NULL,next_generation TEXT NOT NULL,after_sequence TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS token_metadata(chain_id TEXT NOT NULL,address TEXT NOT NULL,symbol TEXT NOT NULL DEFAULT '',decimals INTEGER CHECK(decimals BETWEEN 0 AND 255),retry_at INTEGER NOT NULL DEFAULT 0,PRIMARY KEY(chain_id,address));
CREATE INDEX IF NOT EXISTS token_metadata_retry ON token_metadata(chain_id,retry_at,address);

CREATE TABLE IF NOT EXISTS credit_history_retry(operation_id TEXT PRIMARY KEY,retry_at INTEGER NOT NULL);

CREATE INDEX IF NOT EXISTS public_grant_history_identity ON public_grant_ids(chain_id,protocol_address,LOWER(token_address),LOWER(payer),grant_id,block_number,operation_id);

-- Cache payloads survive invalidation; bindings decide whether they are current.
CREATE TABLE IF NOT EXISTS recognition_sources(scope TEXT PRIMARY KEY,url TEXT NOT NULL,issuer TEXT NOT NULL,provider TEXT NOT NULL DEFAULT '',generation TEXT NOT NULL DEFAULT '',epoch INTEGER NOT NULL);
CREATE TABLE IF NOT EXISTS recognition_bindings(operation_id TEXT PRIMARY KEY,scope TEXT NOT NULL,epoch INTEGER NOT NULL,chain_generation TEXT NOT NULL);
CREATE VIEW IF NOT EXISTS current_recognition AS
SELECT r.operation_id,r.payload,r.updated_at,s.generation AS source_generation
FROM remote_recognition r
JOIN recognition_bindings b ON b.operation_id=r.operation_id
JOIN recognition_sources s ON s.scope=b.scope AND s.epoch=b.epoch
JOIN operations o ON o.id=r.operation_id AND o.canonical=1
WHERE b.chain_generation=COALESCE((SELECT generation FROM chain_generations WHERE id=b.scope),'00000000000000000001');
CREATE TABLE IF NOT EXISTS recognition_attempts(operation_id TEXT PRIMARY KEY,epoch INTEGER NOT NULL,chain_generation TEXT NOT NULL,attempted_at INTEGER NOT NULL);
