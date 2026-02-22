import { OpenAICompatibleProvidersClient } from "@/components/provider-keys/openai-compatible-providers-client";
import { fetchServerData } from "@/lib/server-api";

interface OpenAICompatibleProvidersData {
	providers: {
		id: string;
		createdAt: string;
		updatedAt: string;
		organizationId: string;
		name: string;
		baseUrl: string;
		status: "active" | "inactive" | "deleted";
	}[];
}

export default async function OpenAICompatibleProvidersPage({
	params,
}: {
	params: Promise<{ orgId: string }>;
}) {
	const { orgId } = await params;

	const initialProvidersData =
		await fetchServerData<OpenAICompatibleProvidersData>(
			"GET",
			"/openai-compatible-providers",
			{
				params: {
					query: {
						organizationId: orgId,
					},
				},
			},
		);

	return (
		<OpenAICompatibleProvidersClient
			initialProvidersData={initialProvidersData || undefined}
		/>
	);
}
