CREATE TABLE "openai_compatible_model_alias" (
	"id" text PRIMARY KEY,
	"created_at" timestamp DEFAULT now() NOT NULL,
	"updated_at" timestamp DEFAULT now() NOT NULL,
	"provider_id" text NOT NULL,
	"alias" text NOT NULL,
	"model_id" text NOT NULL,
	"status" text DEFAULT 'active' NOT NULL,
	CONSTRAINT "openai_compatible_model_alias_provider_id_alias_unique" UNIQUE("provider_id","alias")
);
--> statement-breakpoint
CREATE TABLE "openai_compatible_provider" (
	"id" text PRIMARY KEY,
	"created_at" timestamp DEFAULT now() NOT NULL,
	"updated_at" timestamp DEFAULT now() NOT NULL,
	"organization_id" text NOT NULL,
	"name" text NOT NULL,
	"base_url" text NOT NULL,
	"status" text DEFAULT 'active' NOT NULL,
	CONSTRAINT "openai_compatible_provider_organization_id_name_unique" UNIQUE("organization_id","name")
);
--> statement-breakpoint
CREATE TABLE "openai_compatible_provider_key" (
	"id" text PRIMARY KEY,
	"created_at" timestamp DEFAULT now() NOT NULL,
	"updated_at" timestamp DEFAULT now() NOT NULL,
	"provider_id" text NOT NULL,
	"token" text NOT NULL,
	"label" text,
	"status" text DEFAULT 'active' NOT NULL
);
--> statement-breakpoint
CREATE INDEX "openai_compatible_model_alias_provider_id_idx" ON "openai_compatible_model_alias" ("provider_id");--> statement-breakpoint
CREATE INDEX "openai_compatible_model_alias_provider_id_status_idx" ON "openai_compatible_model_alias" ("provider_id","status");--> statement-breakpoint
CREATE INDEX "openai_compatible_provider_organization_id_idx" ON "openai_compatible_provider" ("organization_id");--> statement-breakpoint
CREATE INDEX "openai_compatible_provider_organization_id_status_idx" ON "openai_compatible_provider" ("organization_id","status");--> statement-breakpoint
CREATE INDEX "openai_compatible_provider_key_provider_id_idx" ON "openai_compatible_provider_key" ("provider_id");--> statement-breakpoint
CREATE INDEX "openai_compatible_provider_key_provider_id_status_idx" ON "openai_compatible_provider_key" ("provider_id","status");--> statement-breakpoint
ALTER TABLE "openai_compatible_model_alias" ADD CONSTRAINT "openai_compatible_model_alias_X8E8oWkaoHSc_fkey" FOREIGN KEY ("provider_id") REFERENCES "openai_compatible_provider"("id") ON DELETE CASCADE;--> statement-breakpoint
ALTER TABLE "openai_compatible_provider" ADD CONSTRAINT "openai_compatible_provider_organization_id_organization_id_fkey" FOREIGN KEY ("organization_id") REFERENCES "organization"("id") ON DELETE CASCADE;--> statement-breakpoint
ALTER TABLE "openai_compatible_provider_key" ADD CONSTRAINT "openai_compatible_provider_key_G5uq4Zl6VHgV_fkey" FOREIGN KEY ("provider_id") REFERENCES "openai_compatible_provider"("id") ON DELETE CASCADE;