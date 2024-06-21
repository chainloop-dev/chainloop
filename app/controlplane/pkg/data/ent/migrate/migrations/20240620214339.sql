-- Modify "org_invitations" table
ALTER TABLE "org_invitations" DROP CONSTRAINT "org_invitations_users_sender", ADD CONSTRAINT "org_invitations_users_sender" FOREIGN KEY ("sender_id") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE CASCADE;
