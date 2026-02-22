import { createRoute, OpenAPIHono } from "@hono/zod-openapi";
import { HTTPException } from "hono/http-exception";
import { z } from "zod";

import { maskToken } from "@/lib/maskToken.js";
import { getActiveUserOrganizationIds } from "@/utils/authorization.js";

import { logAuditEvent } from "@llmgateway/audit";
import {
	and,
	db,
	eq,
	ilike,
	inArray,
	ne,
	or,
	shortid,
	tables,
} from "@llmgateway/db";

import type { ServerTypes } from "@/vars.js";

export const openaiCompatibleProviders = new OpenAPIHono<ServerTypes>();

const providerStatusSchema = z.enum(["active", "inactive", "deleted"]);
const keyStatusSchema = z.enum(["active", "inactive", "deleted"]);
const aliasStatusSchema = z.enum(["active", "inactive", "deleted"]);

const providerSchema = z.object({
	id: z.string(),
	createdAt: z.date(),
	updatedAt: z.date(),
	organizationId: z.string(),
	name: z.string(),
	baseUrl: z.string(),
	status: providerStatusSchema,
});

const providerKeySchema = z.object({
	id: z.string(),
	createdAt: z.date(),
	updatedAt: z.date(),
	providerId: z.string(),
	token: z.string(),
	label: z.string().nullable(),
	status: keyStatusSchema,
});

const providerKeyResponseSchema = providerKeySchema
	.omit({ token: true })
	.extend({
		maskedToken: z.string(),
	});

const modelAliasSchema = z.object({
	id: z.string(),
	createdAt: z.date(),
	updatedAt: z.date(),
	providerId: z.string(),
	alias: z.string(),
	modelId: z.string(),
	status: aliasStatusSchema,
});

const createProviderSchema = z.object({
	organizationId: z.string().min(1),
	name: z.string().trim().min(1).max(120),
	baseUrl: z.string().trim().url(),
	status: providerStatusSchema.exclude(["deleted"]).optional(),
});

const listProvidersQuerySchema = z.object({
	organizationId: z.string().optional(),
	search: z.string().trim().optional(),
});

const updateProviderSchema = z
	.object({
		name: z.string().trim().min(1).max(120).optional(),
		baseUrl: z.string().trim().url().optional(),
		status: providerStatusSchema.exclude(["deleted"]).optional(),
	})
	.refine((value) => Object.keys(value).length > 0, {
		message: "At least one field is required",
	});

const createProviderKeySchema = z.object({
	token: z.string().trim().min(1),
	label: z.string().trim().max(120).optional().nullable(),
	status: keyStatusSchema.exclude(["deleted"]).optional(),
});

const updateProviderKeySchema = z
	.object({
		label: z.string().trim().max(120).optional().nullable(),
		status: keyStatusSchema.exclude(["deleted"]).optional(),
	})
	.refine((value) => Object.keys(value).length > 0, {
		message: "At least one field is required",
	});

const createAliasSchema = z.object({
	alias: z
		.string()
		.trim()
		.min(1)
		.max(120)
		.regex(/^[a-zA-Z0-9._:-]+$/, {
			message: "Alias can only contain letters, numbers, ., _, :, and -",
		}),
	modelId: z.string().trim().min(1).max(255),
	status: aliasStatusSchema.exclude(["deleted"]).optional(),
});

const updateAliasSchema = z
	.object({
		alias: z
			.string()
			.trim()
			.min(1)
			.max(120)
			.regex(/^[a-zA-Z0-9._:-]+$/, {
				message: "Alias can only contain letters, numbers, ., _, :, and -",
			})
			.optional(),
		modelId: z.string().trim().min(1).max(255).optional(),
		status: aliasStatusSchema.exclude(["deleted"]).optional(),
	})
	.refine((value) => Object.keys(value).length > 0, {
		message: "At least one field is required",
	});

const listProviderModelsQuerySchema = z.object({
	search: z.string().trim().optional(),
});

const providerPathSchema = z.object({
	id: z.string(),
});

const providerKeyPathSchema = z.object({
	keyId: z.string(),
});

const aliasPathSchema = z.object({
	aliasId: z.string(),
});

async function getAuthorizedOrganizationIds(userId: string) {
	const organizationIds = await getActiveUserOrganizationIds(userId);

	if (!organizationIds.length) {
		throw new HTTPException(403, {
			message: "You don't have access to any organizations",
		});
	}

	return organizationIds;
}

async function getAuthorizedProvider(
	providerId: string,
	organizationIds: string[],
) {
	const provider = await db.query.openaiCompatibleProvider.findFirst({
		where: {
			id: {
				eq: providerId,
			},
			organizationId: {
				in: organizationIds,
			},
			status: {
				ne: "deleted",
			},
		},
	});

	if (!provider) {
		throw new HTTPException(404, {
			message: "OpenAI-compatible provider not found",
		});
	}

	return provider;
}

async function getAuthorizedProviderKey(
	keyId: string,
	organizationIds: string[],
) {
	const providerKey = await db.query.openaiCompatibleProviderKey.findFirst({
		where: {
			id: {
				eq: keyId,
			},
			status: {
				ne: "deleted",
			},
		},
		with: {
			provider: true,
		},
	});

	const provider = providerKey?.provider;
	if (
		!providerKey ||
		!provider ||
		provider.status === "deleted" ||
		!organizationIds.includes(provider.organizationId)
	) {
		throw new HTTPException(404, {
			message: "OpenAI-compatible provider key not found",
		});
	}

	return {
		...providerKey,
		provider,
	};
}

async function getAuthorizedProviderAlias(
	aliasId: string,
	organizationIds: string[],
) {
	const alias = await db.query.openaiCompatibleModelAlias.findFirst({
		where: {
			id: {
				eq: aliasId,
			},
			status: {
				ne: "deleted",
			},
		},
		with: {
			provider: true,
		},
	});

	const provider = alias?.provider;
	if (
		!alias ||
		!provider ||
		provider.status === "deleted" ||
		!organizationIds.includes(provider.organizationId)
	) {
		throw new HTTPException(404, {
			message: "OpenAI-compatible provider alias not found",
		});
	}

	return {
		...alias,
		provider,
	};
}

function getErrorMessage(error: unknown): string {
	if (error instanceof Error) {
		return error.message;
	}

	return "Unknown error";
}

const createProviderRoute = createRoute({
	method: "post",
	path: "/openai-compatible-providers",
	request: {
		body: {
			content: {
				"application/json": {
					schema: createProviderSchema,
				},
			},
		},
	},
	responses: {
		200: {
			content: {
				"application/json": {
					schema: z.object({
						provider: providerSchema.openapi({}),
					}),
				},
			},
			description: "OpenAI-compatible provider created successfully",
		},
		400: {
			content: {
				"application/json": {
					schema: z.object({ message: z.string() }),
				},
			},
			description: "Invalid input",
		},
	},
});

openaiCompatibleProviders.openapi(createProviderRoute, async (c) => {
	const user = c.get("user");
	if (!user) {
		throw new HTTPException(401, { message: "Unauthorized" });
	}

	const { organizationId, name, baseUrl, status } = c.req.valid("json");
	const organizationIds = await getAuthorizedOrganizationIds(user.id);

	if (!organizationIds.includes(organizationId)) {
		throw new HTTPException(403, {
			message: "You don't have access to this organization",
		});
	}

	const normalizedName = name.trim().toLowerCase();
	const normalizedBaseUrl = baseUrl.trim().replace(/\/+$/, "");

	const existingProvider = await db.query.openaiCompatibleProvider.findFirst({
		where: {
			organizationId: { eq: organizationId },
			name: { eq: normalizedName },
			status: { ne: "deleted" },
		},
	});

	if (existingProvider) {
		throw new HTTPException(400, {
			message: `OpenAI-compatible provider '${normalizedName}' already exists for this organization`,
		});
	}

	const [provider] = await db
		.insert(tables.openaiCompatibleProvider)
		.values({
			id: shortid(),
			organizationId,
			name: normalizedName,
			baseUrl: normalizedBaseUrl,
			status: status ?? "active",
		})
		.returning();

	await logAuditEvent({
		organizationId,
		userId: user.id,
		action: "openai_compatible_provider.create",
		resourceType: "openai_compatible_provider",
		resourceId: provider.id,
		metadata: {
			resourceName: provider.name,
			baseUrl: provider.baseUrl,
		},
	});

	return c.json({ provider }, 200);
});

const listProvidersRoute = createRoute({
	method: "get",
	path: "/openai-compatible-providers",
	request: {
		query: listProvidersQuerySchema,
	},
	responses: {
		200: {
			content: {
				"application/json": {
					schema: z.object({
						providers: z.array(providerSchema).openapi({}),
					}),
				},
			},
			description: "List of OpenAI-compatible providers",
		},
	},
});

openaiCompatibleProviders.openapi(listProvidersRoute, async (c) => {
	const user = c.get("user");
	if (!user) {
		throw new HTTPException(401, { message: "Unauthorized" });
	}

	const { organizationId, search } = c.req.valid("query");
	const organizationIds = await getAuthorizedOrganizationIds(user.id);

	if (organizationId && !organizationIds.includes(organizationId)) {
		throw new HTTPException(403, {
			message: "You don't have access to this organization",
		});
	}

	const targetOrganizationIds = organizationId
		? [organizationId]
		: organizationIds;
	const normalizedSearch = search?.trim().toLowerCase();

	const filters = [
		inArray(
			tables.openaiCompatibleProvider.organizationId,
			targetOrganizationIds,
		),
		ne(tables.openaiCompatibleProvider.status, "deleted"),
	];

	if (normalizedSearch) {
		filters.push(
			or(
				ilike(tables.openaiCompatibleProvider.name, `%${normalizedSearch}%`),
				ilike(tables.openaiCompatibleProvider.baseUrl, `%${normalizedSearch}%`),
			)!,
		);
	}

	const providers = await db
		.select()
		.from(tables.openaiCompatibleProvider)
		.where(and(...filters));

	providers.sort((a, b) => {
		const aTime = a.createdAt.getTime();
		const bTime = b.createdAt.getTime();
		if (aTime === bTime) {
			return a.name.localeCompare(b.name);
		}
		return bTime - aTime;
	});

	return c.json({ providers });
});

const updateProviderRoute = createRoute({
	method: "patch",
	path: "/openai-compatible-providers/{id}",
	request: {
		params: providerPathSchema,
		body: {
			content: {
				"application/json": {
					schema: updateProviderSchema,
				},
			},
		},
	},
	responses: {
		200: {
			content: {
				"application/json": {
					schema: z.object({
						message: z.string(),
						provider: providerSchema.openapi({}),
					}),
				},
			},
			description: "OpenAI-compatible provider updated",
		},
	},
});

openaiCompatibleProviders.openapi(updateProviderRoute, async (c) => {
	const user = c.get("user");
	if (!user) {
		throw new HTTPException(401, { message: "Unauthorized" });
	}

	const { id } = c.req.valid("param");
	const updates = c.req.valid("json");
	const organizationIds = await getAuthorizedOrganizationIds(user.id);
	const provider = await getAuthorizedProvider(id, organizationIds);

	const nextName = updates.name?.trim().toLowerCase();
	const nextBaseUrl = updates.baseUrl?.trim().replace(/\/+$/, "");

	if (nextName && nextName !== provider.name) {
		const existingProvider = await db.query.openaiCompatibleProvider.findFirst({
			where: {
				organizationId: { eq: provider.organizationId },
				name: { eq: nextName },
				status: { ne: "deleted" },
			},
		});

		if (existingProvider) {
			throw new HTTPException(400, {
				message: `OpenAI-compatible provider '${nextName}' already exists for this organization`,
			});
		}
	}

	const [updatedProvider] = await db
		.update(tables.openaiCompatibleProvider)
		.set({
			name: nextName ?? provider.name,
			baseUrl: nextBaseUrl ?? provider.baseUrl,
			status: updates.status ?? provider.status,
		})
		.where(eq(tables.openaiCompatibleProvider.id, id))
		.returning();

	await logAuditEvent({
		organizationId: provider.organizationId,
		userId: user.id,
		action: "openai_compatible_provider.update",
		resourceType: "openai_compatible_provider",
		resourceId: id,
		metadata: {
			changes: {
				name: {
					old: provider.name,
					new: updatedProvider.name,
				},
				baseUrl: {
					old: provider.baseUrl,
					new: updatedProvider.baseUrl,
				},
				status: {
					old: provider.status,
					new: updatedProvider.status,
				},
			},
		},
	});

	return c.json({
		message: "OpenAI-compatible provider updated successfully",
		provider: updatedProvider,
	});
});

const deleteProviderRoute = createRoute({
	method: "delete",
	path: "/openai-compatible-providers/{id}",
	request: {
		params: providerPathSchema,
	},
	responses: {
		200: {
			content: {
				"application/json": {
					schema: z.object({ message: z.string() }),
				},
			},
			description: "OpenAI-compatible provider soft-deleted",
		},
	},
});

openaiCompatibleProviders.openapi(deleteProviderRoute, async (c) => {
	const user = c.get("user");
	if (!user) {
		throw new HTTPException(401, { message: "Unauthorized" });
	}

	const { id } = c.req.valid("param");
	const organizationIds = await getAuthorizedOrganizationIds(user.id);
	const provider = await getAuthorizedProvider(id, organizationIds);

	await db
		.update(tables.openaiCompatibleProvider)
		.set({ status: "deleted" })
		.where(eq(tables.openaiCompatibleProvider.id, id));

	await db
		.update(tables.openaiCompatibleProviderKey)
		.set({ status: "deleted" })
		.where(eq(tables.openaiCompatibleProviderKey.providerId, id));

	await db
		.update(tables.openaiCompatibleModelAlias)
		.set({ status: "deleted" })
		.where(eq(tables.openaiCompatibleModelAlias.providerId, id));

	await logAuditEvent({
		organizationId: provider.organizationId,
		userId: user.id,
		action: "openai_compatible_provider.delete",
		resourceType: "openai_compatible_provider",
		resourceId: id,
		metadata: {
			resourceName: provider.name,
		},
	});

	return c.json({
		message: "OpenAI-compatible provider deleted successfully",
	});
});

const createProviderKeyRoute = createRoute({
	method: "post",
	path: "/openai-compatible-providers/{id}/keys",
	request: {
		params: providerPathSchema,
		body: {
			content: {
				"application/json": {
					schema: createProviderKeySchema,
				},
			},
		},
	},
	responses: {
		200: {
			content: {
				"application/json": {
					schema: z.object({
						providerKey: providerKeyResponseSchema.openapi({}),
					}),
				},
			},
			description: "OpenAI-compatible provider key created",
		},
	},
});

openaiCompatibleProviders.openapi(createProviderKeyRoute, async (c) => {
	const user = c.get("user");
	if (!user) {
		throw new HTTPException(401, { message: "Unauthorized" });
	}

	const { id } = c.req.valid("param");
	const { token, label, status } = c.req.valid("json");
	const organizationIds = await getAuthorizedOrganizationIds(user.id);
	const provider = await getAuthorizedProvider(id, organizationIds);

	const [providerKey] = await db
		.insert(tables.openaiCompatibleProviderKey)
		.values({
			id: shortid(),
			providerId: provider.id,
			token: token.trim(),
			label: label?.trim() || null,
			status: status ?? "active",
		})
		.returning();

	await logAuditEvent({
		organizationId: provider.organizationId,
		userId: user.id,
		action: "openai_compatible_provider_key.create",
		resourceType: "openai_compatible_provider_key",
		resourceId: providerKey.id,
		metadata: {
			providerId: provider.id,
			providerName: provider.name,
			label: providerKey.label,
			status: providerKey.status,
		},
	});

	return c.json({
		providerKey: {
			...providerKey,
			maskedToken: maskToken(providerKey.token),
			token: undefined,
		},
	});
});

const listProviderKeysRoute = createRoute({
	method: "get",
	path: "/openai-compatible-providers/{id}/keys",
	request: {
		params: providerPathSchema,
	},
	responses: {
		200: {
			content: {
				"application/json": {
					schema: z.object({
						providerKeys: z.array(providerKeyResponseSchema).openapi({}),
					}),
				},
			},
			description: "List OpenAI-compatible provider keys",
		},
	},
});

openaiCompatibleProviders.openapi(listProviderKeysRoute, async (c) => {
	const user = c.get("user");
	if (!user) {
		throw new HTTPException(401, { message: "Unauthorized" });
	}

	const { id } = c.req.valid("param");
	const organizationIds = await getAuthorizedOrganizationIds(user.id);
	await getAuthorizedProvider(id, organizationIds);

	const providerKeys = await db.query.openaiCompatibleProviderKey.findMany({
		where: {
			providerId: {
				eq: id,
			},
			status: {
				ne: "deleted",
			},
		},
		orderBy: {
			createdAt: "desc",
		},
	});

	return c.json({
		providerKeys: providerKeys.map((providerKey) => ({
			...providerKey,
			maskedToken: maskToken(providerKey.token),
			token: undefined,
		})),
	});
});

const updateProviderKeyRoute = createRoute({
	method: "patch",
	path: "/openai-compatible-providers/keys/{keyId}",
	request: {
		params: providerKeyPathSchema,
		body: {
			content: {
				"application/json": {
					schema: updateProviderKeySchema,
				},
			},
		},
	},
	responses: {
		200: {
			content: {
				"application/json": {
					schema: z.object({
						message: z.string(),
						providerKey: providerKeyResponseSchema.openapi({}),
					}),
				},
			},
			description: "OpenAI-compatible provider key updated",
		},
	},
});

openaiCompatibleProviders.openapi(updateProviderKeyRoute, async (c) => {
	const user = c.get("user");
	if (!user) {
		throw new HTTPException(401, { message: "Unauthorized" });
	}

	const { keyId } = c.req.valid("param");
	const updates = c.req.valid("json");
	const organizationIds = await getAuthorizedOrganizationIds(user.id);
	const providerKey = await getAuthorizedProviderKey(keyId, organizationIds);

	const [updatedProviderKey] = await db
		.update(tables.openaiCompatibleProviderKey)
		.set({
			label:
				updates.label === undefined
					? providerKey.label
					: updates.label?.trim() || null,
			status: updates.status ?? providerKey.status,
		})
		.where(eq(tables.openaiCompatibleProviderKey.id, keyId))
		.returning();

	await logAuditEvent({
		organizationId: providerKey.provider.organizationId,
		userId: user.id,
		action: "openai_compatible_provider_key.update",
		resourceType: "openai_compatible_provider_key",
		resourceId: keyId,
		metadata: {
			providerId: providerKey.providerId,
			providerName: providerKey.provider.name,
			changes: {
				label: {
					old: providerKey.label,
					new: updatedProviderKey.label,
				},
				status: {
					old: providerKey.status,
					new: updatedProviderKey.status,
				},
			},
		},
	});

	return c.json({
		message: "OpenAI-compatible provider key updated successfully",
		providerKey: {
			...updatedProviderKey,
			maskedToken: maskToken(updatedProviderKey.token),
			token: undefined,
		},
	});
});

const deleteProviderKeyRoute = createRoute({
	method: "delete",
	path: "/openai-compatible-providers/keys/{keyId}",
	request: {
		params: providerKeyPathSchema,
	},
	responses: {
		200: {
			content: {
				"application/json": {
					schema: z.object({ message: z.string() }),
				},
			},
			description: "OpenAI-compatible provider key soft-deleted",
		},
	},
});

openaiCompatibleProviders.openapi(deleteProviderKeyRoute, async (c) => {
	const user = c.get("user");
	if (!user) {
		throw new HTTPException(401, { message: "Unauthorized" });
	}

	const { keyId } = c.req.valid("param");
	const organizationIds = await getAuthorizedOrganizationIds(user.id);
	const providerKey = await getAuthorizedProviderKey(keyId, organizationIds);

	await db
		.update(tables.openaiCompatibleProviderKey)
		.set({ status: "deleted" })
		.where(eq(tables.openaiCompatibleProviderKey.id, keyId));

	await logAuditEvent({
		organizationId: providerKey.provider.organizationId,
		userId: user.id,
		action: "openai_compatible_provider_key.delete",
		resourceType: "openai_compatible_provider_key",
		resourceId: keyId,
		metadata: {
			providerId: providerKey.providerId,
			providerName: providerKey.provider.name,
		},
	});

	return c.json({
		message: "OpenAI-compatible provider key deleted successfully",
	});
});

const createAliasRoute = createRoute({
	method: "post",
	path: "/openai-compatible-providers/{id}/aliases",
	request: {
		params: providerPathSchema,
		body: {
			content: {
				"application/json": {
					schema: createAliasSchema,
				},
			},
		},
	},
	responses: {
		200: {
			content: {
				"application/json": {
					schema: z.object({
						alias: modelAliasSchema.openapi({}),
					}),
				},
			},
			description: "OpenAI-compatible model alias created",
		},
	},
});

openaiCompatibleProviders.openapi(createAliasRoute, async (c) => {
	const user = c.get("user");
	if (!user) {
		throw new HTTPException(401, { message: "Unauthorized" });
	}

	const { id } = c.req.valid("param");
	const { alias, modelId, status } = c.req.valid("json");
	const organizationIds = await getAuthorizedOrganizationIds(user.id);
	const provider = await getAuthorizedProvider(id, organizationIds);

	const normalizedAlias = alias.trim().toLowerCase();

	const existingAlias = await db.query.openaiCompatibleModelAlias.findFirst({
		where: {
			providerId: {
				eq: provider.id,
			},
			alias: {
				eq: normalizedAlias,
			},
			status: {
				ne: "deleted",
			},
		},
	});

	if (existingAlias) {
		throw new HTTPException(400, {
			message: `Alias '${normalizedAlias}' already exists for provider '${provider.name}'`,
		});
	}

	const [createdAlias] = await db
		.insert(tables.openaiCompatibleModelAlias)
		.values({
			id: shortid(),
			providerId: provider.id,
			alias: normalizedAlias,
			modelId: modelId.trim(),
			status: status ?? "active",
		})
		.returning();

	await logAuditEvent({
		organizationId: provider.organizationId,
		userId: user.id,
		action: "openai_compatible_model_alias.create",
		resourceType: "openai_compatible_model_alias",
		resourceId: createdAlias.id,
		metadata: {
			providerId: provider.id,
			providerName: provider.name,
			alias: createdAlias.alias,
			modelId: createdAlias.modelId,
		},
	});

	return c.json({ alias: createdAlias });
});

const listAliasesRoute = createRoute({
	method: "get",
	path: "/openai-compatible-providers/{id}/aliases",
	request: {
		params: providerPathSchema,
	},
	responses: {
		200: {
			content: {
				"application/json": {
					schema: z.object({
						aliases: z.array(modelAliasSchema).openapi({}),
					}),
				},
			},
			description: "List OpenAI-compatible model aliases",
		},
	},
});

openaiCompatibleProviders.openapi(listAliasesRoute, async (c) => {
	const user = c.get("user");
	if (!user) {
		throw new HTTPException(401, { message: "Unauthorized" });
	}

	const { id } = c.req.valid("param");
	const organizationIds = await getAuthorizedOrganizationIds(user.id);
	await getAuthorizedProvider(id, organizationIds);

	const aliases = await db.query.openaiCompatibleModelAlias.findMany({
		where: {
			providerId: {
				eq: id,
			},
			status: {
				ne: "deleted",
			},
		},
		orderBy: {
			createdAt: "desc",
		},
	});

	return c.json({ aliases });
});

const updateAliasRoute = createRoute({
	method: "patch",
	path: "/openai-compatible-providers/aliases/{aliasId}",
	request: {
		params: aliasPathSchema,
		body: {
			content: {
				"application/json": {
					schema: updateAliasSchema,
				},
			},
		},
	},
	responses: {
		200: {
			content: {
				"application/json": {
					schema: z.object({
						message: z.string(),
						alias: modelAliasSchema.openapi({}),
					}),
				},
			},
			description: "OpenAI-compatible model alias updated",
		},
	},
});

openaiCompatibleProviders.openapi(updateAliasRoute, async (c) => {
	const user = c.get("user");
	if (!user) {
		throw new HTTPException(401, { message: "Unauthorized" });
	}

	const { aliasId } = c.req.valid("param");
	const updates = c.req.valid("json");
	const organizationIds = await getAuthorizedOrganizationIds(user.id);
	const alias = await getAuthorizedProviderAlias(aliasId, organizationIds);

	const nextAlias = updates.alias?.trim().toLowerCase();

	if (nextAlias && nextAlias !== alias.alias) {
		const existingAlias = await db.query.openaiCompatibleModelAlias.findFirst({
			where: {
				providerId: { eq: alias.providerId },
				alias: { eq: nextAlias },
				status: { ne: "deleted" },
			},
		});

		if (existingAlias) {
			throw new HTTPException(400, {
				message: `Alias '${nextAlias}' already exists for provider '${alias.provider.name}'`,
			});
		}
	}

	const [updatedAlias] = await db
		.update(tables.openaiCompatibleModelAlias)
		.set({
			alias: nextAlias ?? alias.alias,
			modelId: updates.modelId?.trim() ?? alias.modelId,
			status: updates.status ?? alias.status,
		})
		.where(eq(tables.openaiCompatibleModelAlias.id, aliasId))
		.returning();

	await logAuditEvent({
		organizationId: alias.provider.organizationId,
		userId: user.id,
		action: "openai_compatible_model_alias.update",
		resourceType: "openai_compatible_model_alias",
		resourceId: aliasId,
		metadata: {
			providerId: alias.providerId,
			providerName: alias.provider.name,
			changes: {
				alias: {
					old: alias.alias,
					new: updatedAlias.alias,
				},
				modelId: {
					old: alias.modelId,
					new: updatedAlias.modelId,
				},
				status: {
					old: alias.status,
					new: updatedAlias.status,
				},
			},
		},
	});

	return c.json({
		message: "OpenAI-compatible model alias updated successfully",
		alias: updatedAlias,
	});
});

const deleteAliasRoute = createRoute({
	method: "delete",
	path: "/openai-compatible-providers/aliases/{aliasId}",
	request: {
		params: aliasPathSchema,
	},
	responses: {
		200: {
			content: {
				"application/json": {
					schema: z.object({ message: z.string() }),
				},
			},
			description: "OpenAI-compatible model alias soft-deleted",
		},
	},
});

openaiCompatibleProviders.openapi(deleteAliasRoute, async (c) => {
	const user = c.get("user");
	if (!user) {
		throw new HTTPException(401, { message: "Unauthorized" });
	}

	const { aliasId } = c.req.valid("param");
	const organizationIds = await getAuthorizedOrganizationIds(user.id);
	const alias = await getAuthorizedProviderAlias(aliasId, organizationIds);

	await db
		.update(tables.openaiCompatibleModelAlias)
		.set({ status: "deleted" })
		.where(eq(tables.openaiCompatibleModelAlias.id, aliasId));

	await logAuditEvent({
		organizationId: alias.provider.organizationId,
		userId: user.id,
		action: "openai_compatible_model_alias.delete",
		resourceType: "openai_compatible_model_alias",
		resourceId: aliasId,
		metadata: {
			providerId: alias.providerId,
			providerName: alias.provider.name,
			alias: alias.alias,
		},
	});

	return c.json({
		message: "OpenAI-compatible model alias deleted successfully",
	});
});

const listProviderModelsRoute = createRoute({
	method: "get",
	path: "/openai-compatible-providers/{id}/models",
	request: {
		params: providerPathSchema,
		query: listProviderModelsQuerySchema,
	},
	responses: {
		200: {
			content: {
				"application/json": {
					schema: z.object({
						models: z.array(z.object({ id: z.string() })).openapi({}),
					}),
				},
			},
			description: "List models from provider /v1/models endpoint",
		},
		502: {
			content: {
				"application/json": {
					schema: z.object({ message: z.string() }),
				},
			},
			description: "Upstream provider request failed",
		},
	},
});

openaiCompatibleProviders.openapi(listProviderModelsRoute, async (c) => {
	const user = c.get("user");
	if (!user) {
		throw new HTTPException(401, { message: "Unauthorized" });
	}

	const { id } = c.req.valid("param");
	const { search } = c.req.valid("query");
	const organizationIds = await getAuthorizedOrganizationIds(user.id);
	const provider = await getAuthorizedProvider(id, organizationIds);

	const activeKeys = await db.query.openaiCompatibleProviderKey.findMany({
		where: {
			providerId: {
				eq: provider.id,
			},
			status: {
				eq: "active",
			},
		},
		orderBy: {
			createdAt: "asc",
		},
	});

	const selectedKey = activeKeys[0];

	if (!selectedKey) {
		throw new HTTPException(400, {
			message: "No active key available for this provider",
		});
	}

	const normalizedBaseUrl = provider.baseUrl.replace(/\/+$/, "");
	const modelsUrl = `${normalizedBaseUrl}/v1/models`;

	let response: Response;
	try {
		response = await fetch(modelsUrl, {
			method: "GET",
			headers: {
				Authorization: `Bearer ${selectedKey.token}`,
				"Content-Type": "application/json",
			},
		});
	} catch (error) {
		throw new HTTPException(502, {
			message: `Failed to reach provider: ${getErrorMessage(error)}`,
		});
	}

	if (!response.ok) {
		const errorText = await response.text();
		throw new HTTPException(502, {
			message: `Provider /v1/models returned ${response.status}: ${errorText || response.statusText}`,
		});
	}

	let payload: unknown;
	try {
		payload = await response.json();
	} catch (error) {
		throw new HTTPException(502, {
			message: `Provider returned invalid JSON: ${getErrorMessage(error)}`,
		});
	}

	const normalizedModelsPayload = z.object({
		data: z.array(
			z.object({
				id: z.string(),
			}),
		),
	});

	const parsed = normalizedModelsPayload.safeParse(payload);
	if (!parsed.success) {
		throw new HTTPException(502, {
			message: "Provider returned unexpected /v1/models response format",
		});
	}

	const normalizedSearch = search?.trim().toLowerCase();
	const models = parsed.data.data
		.map((model) => ({ id: model.id }))
		.filter((model) =>
			normalizedSearch
				? model.id.toLowerCase().includes(normalizedSearch)
				: true,
		)
		.sort((a, b) => a.id.localeCompare(b.id));

	return c.json({ models }, 200);
});

export default openaiCompatibleProviders;
