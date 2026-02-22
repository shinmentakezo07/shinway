import { afterEach, beforeEach, describe, expect, test, vi } from "vitest";

import { app } from "@/index.js";
import { createTestUser, deleteAll } from "@/testing.js";

import { db, tables } from "@llmgateway/db";

describe("openai-compatible providers route", () => {
	let token = "";

	beforeEach(async () => {
		token = await createTestUser();

		await db.insert(tables.organization).values({
			id: "test-org-id",
			name: "Test Organization",
			billingEmail: "test@example.com",
		});

		await db.insert(tables.userOrganization).values({
			userId: "test-user-id",
			organizationId: "test-org-id",
			role: "owner",
		});

		await db.insert(tables.openaiCompatibleProvider).values({
			id: "test-openai-compatible-provider-id",
			organizationId: "test-org-id",
			name: "custom-openai",
			baseUrl: "https://custom-openai.example.com",
			status: "active",
		});

		await db.insert(tables.openaiCompatibleProviderKey).values({
			id: "test-openai-compatible-provider-key-id",
			providerId: "test-openai-compatible-provider-id",
			token: "test-provider-token",
			label: "primary",
			status: "active",
		});

		await db.insert(tables.openaiCompatibleModelAlias).values({
			id: "test-openai-compatible-model-alias-id",
			providerId: "test-openai-compatible-provider-id",
			alias: "cheap",
			modelId: "gpt-4o-mini",
			status: "active",
		});
	});

	afterEach(async () => {
		await deleteAll();
	});

	test("GET /openai-compatible-providers unauthorized", async () => {
		const res = await app.request("/openai-compatible-providers");
		expect(res.status).toBe(401);
	});

	test("POST /openai-compatible-providers", async () => {
		const res = await app.request("/openai-compatible-providers", {
			method: "POST",
			headers: {
				"Content-Type": "application/json",
				Cookie: token,
			},
			body: JSON.stringify({
				organizationId: "test-org-id",
				name: "new-provider",
				baseUrl: "https://new-provider.example.com",
			}),
		});

		expect(res.status).toBe(200);
		const json = await res.json();
		expect(json.provider.name).toBe("new-provider");
		expect(json.provider.baseUrl).toBe("https://new-provider.example.com");

		const provider = await db.query.openaiCompatibleProvider.findFirst({
			where: {
				name: {
					eq: "new-provider",
				},
			},
		});
		expect(provider).not.toBeNull();
	});

	test("GET /openai-compatible-providers", async () => {
		const res = await app.request("/openai-compatible-providers", {
			headers: {
				Cookie: token,
			},
		});

		expect(res.status).toBe(200);
		const json = await res.json();
		expect(json.providers).toHaveLength(1);
		expect(json.providers[0].name).toBe("custom-openai");
	});

	test("PATCH /openai-compatible-providers/{id}", async () => {
		const res = await app.request(
			"/openai-compatible-providers/test-openai-compatible-provider-id",
			{
				method: "PATCH",
				headers: {
					"Content-Type": "application/json",
					Cookie: token,
				},
				body: JSON.stringify({
					name: "updated-provider",
					status: "inactive",
				}),
			},
		);

		expect(res.status).toBe(200);
		const json = await res.json();
		expect(json.provider.name).toBe("updated-provider");
		expect(json.provider.status).toBe("inactive");
	});

	test("DELETE /openai-compatible-providers/{id}", async () => {
		const res = await app.request(
			"/openai-compatible-providers/test-openai-compatible-provider-id",
			{
				method: "DELETE",
				headers: {
					Cookie: token,
				},
			},
		);

		expect(res.status).toBe(200);
		const json = await res.json();
		expect(json.message).toBe(
			"OpenAI-compatible provider deleted successfully",
		);
	});

	test("POST /openai-compatible-providers/{id}/keys", async () => {
		const res = await app.request(
			"/openai-compatible-providers/test-openai-compatible-provider-id/keys",
			{
				method: "POST",
				headers: {
					"Content-Type": "application/json",
					Cookie: token,
				},
				body: JSON.stringify({
					token: "another-provider-token",
					label: "backup",
				}),
			},
		);

		expect(res.status).toBe(200);
		const json = await res.json();
		expect(json.providerKey.label).toBe("backup");
		expect(json.providerKey.maskedToken).toContain("another-prov");
	});

	test("GET /openai-compatible-providers/{id}/keys", async () => {
		const res = await app.request(
			"/openai-compatible-providers/test-openai-compatible-provider-id/keys",
			{
				headers: {
					Cookie: token,
				},
			},
		);

		expect(res.status).toBe(200);
		const json = await res.json();
		expect(json.providerKeys).toHaveLength(1);
		expect(json.providerKeys[0].id).toBe(
			"test-openai-compatible-provider-key-id",
		);
		expect(json.providerKeys[0].maskedToken).toContain("test-provide");
	});

	test("PATCH /openai-compatible-providers/keys/{keyId}", async () => {
		const res = await app.request(
			"/openai-compatible-providers/keys/test-openai-compatible-provider-key-id",
			{
				method: "PATCH",
				headers: {
					"Content-Type": "application/json",
					Cookie: token,
				},
				body: JSON.stringify({
					status: "inactive",
					label: "updated",
				}),
			},
		);

		expect(res.status).toBe(200);
		const json = await res.json();
		expect(json.providerKey.status).toBe("inactive");
		expect(json.providerKey.label).toBe("updated");
	});

	test("DELETE /openai-compatible-providers/keys/{keyId}", async () => {
		const res = await app.request(
			"/openai-compatible-providers/keys/test-openai-compatible-provider-key-id",
			{
				method: "DELETE",
				headers: {
					Cookie: token,
				},
			},
		);

		expect(res.status).toBe(200);
		const json = await res.json();
		expect(json.message).toBe(
			"OpenAI-compatible provider key deleted successfully",
		);
	});

	test("POST /openai-compatible-providers/{id}/aliases", async () => {
		const res = await app.request(
			"/openai-compatible-providers/test-openai-compatible-provider-id/aliases",
			{
				method: "POST",
				headers: {
					"Content-Type": "application/json",
					Cookie: token,
				},
				body: JSON.stringify({
					alias: "reasoning",
					modelId: "gpt-4.1",
				}),
			},
		);

		expect(res.status).toBe(200);
		const json = await res.json();
		expect(json.alias.alias).toBe("reasoning");
		expect(json.alias.modelId).toBe("gpt-4.1");
	});

	test("GET /openai-compatible-providers/{id}/aliases", async () => {
		const res = await app.request(
			"/openai-compatible-providers/test-openai-compatible-provider-id/aliases",
			{
				headers: {
					Cookie: token,
				},
			},
		);

		expect(res.status).toBe(200);
		const json = await res.json();
		expect(json.aliases).toHaveLength(1);
		expect(json.aliases[0].alias).toBe("cheap");
	});

	test("PATCH /openai-compatible-providers/aliases/{aliasId}", async () => {
		const res = await app.request(
			"/openai-compatible-providers/aliases/test-openai-compatible-model-alias-id",
			{
				method: "PATCH",
				headers: {
					"Content-Type": "application/json",
					Cookie: token,
				},
				body: JSON.stringify({
					alias: "cheapest",
					modelId: "gpt-4.1-mini",
				}),
			},
		);

		expect(res.status).toBe(200);
		const json = await res.json();
		expect(json.alias.alias).toBe("cheapest");
		expect(json.alias.modelId).toBe("gpt-4.1-mini");
	});

	test("DELETE /openai-compatible-providers/aliases/{aliasId}", async () => {
		const res = await app.request(
			"/openai-compatible-providers/aliases/test-openai-compatible-model-alias-id",
			{
				method: "DELETE",
				headers: {
					Cookie: token,
				},
			},
		);

		expect(res.status).toBe(200);
		const json = await res.json();
		expect(json.message).toBe(
			"OpenAI-compatible model alias deleted successfully",
		);
	});

	test("GET /openai-compatible-providers/{id}/models with search", async () => {
		const fetchMock = vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
			new Response(
				JSON.stringify({
					data: [
						{ id: "gpt-4o" },
						{ id: "gpt-4o-mini" },
						{ id: "claude-sonnet-4.5" },
					],
				}),
				{
					status: 200,
					headers: {
						"Content-Type": "application/json",
					},
				},
			),
		);

		const res = await app.request(
			"/openai-compatible-providers/test-openai-compatible-provider-id/models?search=4o",
			{
				headers: {
					Cookie: token,
				},
			},
		);

		expect(res.status).toBe(200);
		const json = await res.json();
		expect(json.models).toEqual([{ id: "gpt-4o" }, { id: "gpt-4o-mini" }]);

		fetchMock.mockRestore();
	});
});
