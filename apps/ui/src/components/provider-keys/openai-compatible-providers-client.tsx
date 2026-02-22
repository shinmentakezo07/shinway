"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { useQueryClient } from "@tanstack/react-query";
import { format } from "date-fns";
import {
	AlertTriangle,
	Check,
	Copy,
	Loader2,
	Pencil,
	Plus,
	RefreshCw,
	Search,
	Trash,
} from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { useForm } from "react-hook-form";
import { z } from "zod";

import { useDashboardNavigation } from "@/hooks/useDashboardNavigation";
import { Button } from "@/lib/components/button";
import {
	Card,
	CardContent,
	CardHeader,
	CardTitle,
} from "@/lib/components/card";
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogFooter,
	DialogHeader,
	DialogTitle,
	DialogTrigger,
} from "@/lib/components/dialog";
import {
	Form,
	FormControl,
	FormField,
	FormItem,
	FormLabel,
	FormMessage,
} from "@/lib/components/form";
import { Input } from "@/lib/components/input";
import { StatusBadge } from "@/lib/components/status-badge";
import {
	Table,
	TableBody,
	TableCell,
	TableHead,
	TableHeader,
	TableRow,
} from "@/lib/components/table";
import {
	Tabs,
	TabsContent,
	TabsList,
	TabsTrigger,
} from "@/lib/components/tabs";
import { toast } from "@/lib/components/use-toast";
import { useApi } from "@/lib/fetch-client";

const providerCreateSchema = z.object({
	name: z.string().trim().min(1, "Provider name is required").max(120),
	baseUrl: z.string().trim().url("Enter a valid URL"),
	status: z.enum(["active", "inactive"]),
});

const providerUpdateSchema = z
	.object({
		name: z.string().trim().min(1).max(120).optional(),
		baseUrl: z.string().trim().url().optional(),
		status: z.enum(["active", "inactive"]).optional(),
	})
	.refine((value) => Object.keys(value).length > 0, {
		message: "At least one field is required",
	});

const providerKeyCreateSchema = z.object({
	token: z.string().trim().min(1, "API key is required"),
	label: z.string().trim().max(120).optional(),
	status: z.enum(["active", "inactive"]),
});

const providerKeyUpdateSchema = z
	.object({
		label: z.string().trim().max(120).optional(),
		status: z.enum(["active", "inactive"]).optional(),
	})
	.refine((value) => Object.keys(value).length > 0, {
		message: "At least one field is required",
	});

const aliasCreateSchema = z.object({
	alias: z
		.string()
		.trim()
		.min(1, "Alias is required")
		.max(120)
		.regex(/^[a-zA-Z0-9._:-]+$/, {
			message: "Alias can only contain letters, numbers, ., _, :, and -",
		}),
	modelId: z.string().trim().min(1, "Model ID is required").max(255),
	status: z.enum(["active", "inactive"]),
});

const aliasUpdateSchema = z
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
		status: z.enum(["active", "inactive"]).optional(),
	})
	.refine((value) => Object.keys(value).length > 0, {
		message: "At least one field is required",
	});

type CreateProviderFormValues = z.infer<typeof providerCreateSchema>;
type UpdateProviderFormValues = z.infer<typeof providerUpdateSchema>;
type CreateProviderKeyFormValues = z.infer<typeof providerKeyCreateSchema>;
type UpdateProviderKeyFormValues = z.infer<typeof providerKeyUpdateSchema>;
type CreateAliasFormValues = z.infer<typeof aliasCreateSchema>;
type UpdateAliasFormValues = z.infer<typeof aliasUpdateSchema>;

interface OpenAICompatibleProvider {
	id: string;
	createdAt: string;
	updatedAt: string;
	organizationId: string;
	name: string;
	baseUrl: string;
	status: "active" | "inactive" | "deleted";
}

interface OpenAICompatibleModel {
	id: string;
}

interface OpenAICompatibleProvidersClientProps {
	initialProvidersData?: {
		providers: OpenAICompatibleProvider[];
	};
}

interface CreateProviderDialogProps {
	organizationId: string;
	onCreated: (providerId: string) => void;
}

function copyText(value: string, successMessage: string) {
	navigator.clipboard
		.writeText(value)
		.then(() => {
			toast({ title: successMessage });
		})
		.catch(() => {
			toast({
				title: "Copy failed",
				description: "Unable to copy to clipboard.",
				variant: "destructive",
			});
		});
}

function useDebouncedValue(value: string, delayMs = 300) {
	const [debouncedValue, setDebouncedValue] = useState(value);

	useEffect(() => {
		const timeoutId = window.setTimeout(() => {
			setDebouncedValue(value.trim());
		}, delayMs);

		return () => {
			window.clearTimeout(timeoutId);
		};
	}, [value, delayMs]);

	return debouncedValue;
}

function formatTimestamp(value: string) {
	const date = new Date(value);
	if (Number.isNaN(date.getTime())) {
		return "-";
	}

	return format(date, "yyyy-MM-dd HH:mm");
}

function getOpenApiErrorMessage(error: unknown, fallback: string) {
	if (typeof error === "object" && error !== null) {
		const err = error as Record<string, unknown>;
		if (err.error && typeof err.error === "object") {
			const nestedError = err.error as Record<string, unknown>;
			if (typeof nestedError.message === "string") {
				return nestedError.message;
			}
		}
		if (typeof err.message === "string") {
			return err.message;
		}
	}

	if (error instanceof Error && error.message) {
		return error.message;
	}

	return fallback;
}

function ProviderCreateDialog({
	organizationId,
	onCreated,
}: CreateProviderDialogProps) {
	const api = useApi();
	const queryClient = useQueryClient();
	const [open, setOpen] = useState(false);

	const form = useForm<CreateProviderFormValues>({
		resolver: zodResolver(providerCreateSchema),
		defaultValues: {
			name: "",
			baseUrl: "",
			status: "active",
		},
	});

	const createProviderMutation = api.useMutation(
		"post",
		"/openai-compatible-providers",
	);

	const onSubmit = async (values: CreateProviderFormValues) => {
		try {
			const response = await createProviderMutation.mutateAsync({
				body: {
					organizationId,
					name: values.name,
					baseUrl: values.baseUrl,
					status: values.status,
				},
			});

			const providersQueryKey = api.queryOptions(
				"get",
				"/openai-compatible-providers",
				{
					params: {
						query: {
							organizationId,
						},
					},
				},
			).queryKey;
			await queryClient.invalidateQueries({ queryKey: providersQueryKey });

			toast({
				title: "Provider created",
				description: `Added '${response.provider.name}'.`,
			});
			onCreated(response.provider.id);
			setOpen(false);
			form.reset({ name: "", baseUrl: "", status: "active" });
		} catch (error) {
			toast({
				title: "Failed to create provider",
				description: getOpenApiErrorMessage(error, "Please try again."),
				variant: "destructive",
			});
		}
	};

	return (
		<Dialog
			open={open}
			onOpenChange={(nextOpen) => {
				setOpen(nextOpen);
				if (!nextOpen) {
					form.reset({ name: "", baseUrl: "", status: "active" });
				}
			}}
		>
			<DialogTrigger asChild>
				<Button>
					<Plus className="mr-2 h-4 w-4" />
					Add Provider
				</Button>
			</DialogTrigger>
			<DialogContent className="sm:max-w-[520px]">
				<DialogHeader>
					<DialogTitle>Add OpenAI-Compatible Provider</DialogTitle>
					<DialogDescription>
						Create a provider target with a base URL and then attach keys and
						aliases.
					</DialogDescription>
				</DialogHeader>
				<Form {...form}>
					<form onSubmit={form.handleSubmit(onSubmit)} className="space-y-4">
						<FormField
							control={form.control}
							name="name"
							render={({ field }) => (
								<FormItem>
									<FormLabel>Provider name</FormLabel>
									<FormControl>
										<Input placeholder="my-provider" {...field} />
									</FormControl>
									<FormMessage />
								</FormItem>
							)}
						/>
						<FormField
							control={form.control}
							name="baseUrl"
							render={({ field }) => (
								<FormItem>
									<FormLabel>Base URL</FormLabel>
									<FormControl>
										<Input placeholder="https://api.example.com" {...field} />
									</FormControl>
									<FormMessage />
								</FormItem>
							)}
						/>
						<FormField
							control={form.control}
							name="status"
							render={({ field }) => (
								<FormItem>
									<FormLabel>Status</FormLabel>
									<FormControl>
										<div className="flex items-center gap-2">
											<Button
												type="button"
												variant={
													field.value === "active" ? "default" : "outline"
												}
												onClick={() => field.onChange("active")}
											>
												Active
											</Button>
											<Button
												type="button"
												variant={
													field.value === "inactive" ? "default" : "outline"
												}
												onClick={() => field.onChange("inactive")}
											>
												Inactive
											</Button>
										</div>
									</FormControl>
									<FormMessage />
								</FormItem>
							)}
						/>
						<DialogFooter>
							<Button
								type="button"
								variant="outline"
								onClick={() => setOpen(false)}
							>
								Cancel
							</Button>
							<Button type="submit" disabled={createProviderMutation.isPending}>
								{createProviderMutation.isPending ? (
									<>
										<Loader2 className="mr-2 h-4 w-4 animate-spin" />
										Creating
									</>
								) : (
									"Create"
								)}
							</Button>
						</DialogFooter>
					</form>
				</Form>
			</DialogContent>
		</Dialog>
	);
}

interface ProviderUpdateDialogProps {
	provider: OpenAICompatibleProvider;
	onUpdated: (provider: OpenAICompatibleProvider) => void;
}

function ProviderUpdateDialog({
	provider,
	onUpdated,
}: ProviderUpdateDialogProps) {
	const api = useApi();
	const queryClient = useQueryClient();
	const [open, setOpen] = useState(false);

	const form = useForm<UpdateProviderFormValues>({
		resolver: zodResolver(providerUpdateSchema),
		defaultValues: {
			name: provider.name,
			baseUrl: provider.baseUrl,
			status: provider.status === "inactive" ? "inactive" : "active",
		},
	});

	const updateProviderMutation = api.useMutation(
		"patch",
		"/openai-compatible-providers/{id}",
	);

	const onSubmit = async (values: UpdateProviderFormValues) => {
		try {
			const response = await updateProviderMutation.mutateAsync({
				params: {
					path: {
						id: provider.id,
					},
				},
				body: {
					name: values.name,
					baseUrl: values.baseUrl,
					status: values.status,
				},
			});

			const providersQueryKey = api.queryOptions(
				"get",
				"/openai-compatible-providers",
				{
					params: {
						query: {
							organizationId: provider.organizationId,
						},
					},
				},
			).queryKey;
			await queryClient.invalidateQueries({ queryKey: providersQueryKey });

			onUpdated(response.provider);
			setOpen(false);
			toast({ title: "Provider updated" });
		} catch (error) {
			toast({
				title: "Failed to update provider",
				description: getOpenApiErrorMessage(error, "Please try again."),
				variant: "destructive",
			});
		}
	};

	return (
		<Dialog
			open={open}
			onOpenChange={(nextOpen) => {
				setOpen(nextOpen);
				if (nextOpen) {
					form.reset({
						name: provider.name,
						baseUrl: provider.baseUrl,
						status: provider.status === "inactive" ? "inactive" : "active",
					});
				}
			}}
		>
			<DialogTrigger asChild>
				<Button variant="outline" size="sm">
					<Pencil className="h-4 w-4" />
					<span className="sr-only">Edit provider</span>
				</Button>
			</DialogTrigger>
			<DialogContent className="sm:max-w-[520px]">
				<DialogHeader>
					<DialogTitle>Edit Provider</DialogTitle>
					<DialogDescription>
						Update provider settings for {provider.name}.
					</DialogDescription>
				</DialogHeader>
				<Form {...form}>
					<form onSubmit={form.handleSubmit(onSubmit)} className="space-y-4">
						<FormField
							control={form.control}
							name="name"
							render={({ field }) => (
								<FormItem>
									<FormLabel>Provider name</FormLabel>
									<FormControl>
										<Input {...field} />
									</FormControl>
									<FormMessage />
								</FormItem>
							)}
						/>
						<FormField
							control={form.control}
							name="baseUrl"
							render={({ field }) => (
								<FormItem>
									<FormLabel>Base URL</FormLabel>
									<FormControl>
										<Input {...field} />
									</FormControl>
									<FormMessage />
								</FormItem>
							)}
						/>
						<FormField
							control={form.control}
							name="status"
							render={({ field }) => (
								<FormItem>
									<FormLabel>Status</FormLabel>
									<FormControl>
										<div className="flex items-center gap-2">
											<Button
												type="button"
												variant={
													field.value === "active" ? "default" : "outline"
												}
												onClick={() => field.onChange("active")}
											>
												Active
											</Button>
											<Button
												type="button"
												variant={
													field.value === "inactive" ? "default" : "outline"
												}
												onClick={() => field.onChange("inactive")}
											>
												Inactive
											</Button>
										</div>
									</FormControl>
									<FormMessage />
								</FormItem>
							)}
						/>
						<DialogFooter>
							<Button
								type="button"
								variant="outline"
								onClick={() => setOpen(false)}
							>
								Cancel
							</Button>
							<Button type="submit" disabled={updateProviderMutation.isPending}>
								{updateProviderMutation.isPending ? (
									<>
										<Loader2 className="mr-2 h-4 w-4 animate-spin" />
										Saving
									</>
								) : (
									"Save"
								)}
							</Button>
						</DialogFooter>
					</form>
				</Form>
			</DialogContent>
		</Dialog>
	);
}

interface ProviderDeleteButtonProps {
	provider: OpenAICompatibleProvider;
	onDeleted: (providerId: string) => void;
}

function ProviderDeleteButton({
	provider,
	onDeleted,
}: ProviderDeleteButtonProps) {
	const api = useApi();
	const queryClient = useQueryClient();

	const deleteProviderMutation = api.useMutation(
		"delete",
		"/openai-compatible-providers/{id}",
	);

	const handleDelete = async () => {
		try {
			await deleteProviderMutation.mutateAsync({
				params: {
					path: {
						id: provider.id,
					},
				},
			});

			const providersQueryKey = api.queryOptions(
				"get",
				"/openai-compatible-providers",
				{
					params: {
						query: {
							organizationId: provider.organizationId,
						},
					},
				},
			).queryKey;
			await queryClient.invalidateQueries({ queryKey: providersQueryKey });

			onDeleted(provider.id);
			toast({ title: "Provider deleted" });
		} catch (error) {
			toast({
				title: "Failed to delete provider",
				description: getOpenApiErrorMessage(error, "Please try again."),
				variant: "destructive",
			});
		}
	};

	return (
		<Button
			variant="outline"
			size="sm"
			onClick={handleDelete}
			disabled={deleteProviderMutation.isPending}
		>
			{deleteProviderMutation.isPending ? (
				<Loader2 className="h-4 w-4 animate-spin" />
			) : (
				<Trash className="h-4 w-4" />
			)}
			<span className="sr-only">Delete provider</span>
		</Button>
	);
}

interface ProviderKeysSectionProps {
	provider: OpenAICompatibleProvider;
}

function ProviderKeysSection({ provider }: ProviderKeysSectionProps) {
	const api = useApi();
	const queryClient = useQueryClient();
	const [openCreate, setOpenCreate] = useState(false);
	const [editingKeyId, setEditingKeyId] = useState<string | null>(null);

	const createForm = useForm<CreateProviderKeyFormValues>({
		resolver: zodResolver(providerKeyCreateSchema),
		defaultValues: {
			token: "",
			label: "",
			status: "active",
		},
	});

	const editForm = useForm<UpdateProviderKeyFormValues>({
		resolver: zodResolver(providerKeyUpdateSchema),
		defaultValues: {
			label: "",
			status: "active",
		},
	});

	const providerKeysQuery = api.useQuery(
		"get",
		"/openai-compatible-providers/{id}/keys",
		{
			params: {
				path: {
					id: provider.id,
				},
			},
		},
	);

	const createKeyMutation = api.useMutation(
		"post",
		"/openai-compatible-providers/{id}/keys",
	);
	const updateKeyMutation = api.useMutation(
		"patch",
		"/openai-compatible-providers/keys/{keyId}",
	);
	const deleteKeyMutation = api.useMutation(
		"delete",
		"/openai-compatible-providers/keys/{keyId}",
	);

	const keys = providerKeysQuery.data?.providerKeys ?? [];

	const keysQueryKey = api.queryOptions(
		"get",
		"/openai-compatible-providers/{id}/keys",
		{
			params: {
				path: {
					id: provider.id,
				},
			},
		},
	).queryKey;

	const onCreateKey = async (values: CreateProviderKeyFormValues) => {
		try {
			await createKeyMutation.mutateAsync({
				params: {
					path: {
						id: provider.id,
					},
				},
				body: {
					token: values.token,
					label: values.label?.trim() || null,
					status: values.status,
				},
			});
			await queryClient.invalidateQueries({ queryKey: keysQueryKey });
			toast({ title: "Key added" });
			setOpenCreate(false);
			createForm.reset({ token: "", label: "", status: "active" });
		} catch (error) {
			toast({
				title: "Failed to add key",
				description: getOpenApiErrorMessage(error, "Please try again."),
				variant: "destructive",
			});
		}
	};

	const onEditKey = async (
		keyId: string,
		values: UpdateProviderKeyFormValues,
	) => {
		try {
			await updateKeyMutation.mutateAsync({
				params: {
					path: {
						keyId,
					},
				},
				body: {
					label: values.label?.trim() || null,
					status: values.status,
				},
			});
			await queryClient.invalidateQueries({ queryKey: keysQueryKey });
			toast({ title: "Key updated" });
			setEditingKeyId(null);
		} catch (error) {
			toast({
				title: "Failed to update key",
				description: getOpenApiErrorMessage(error, "Please try again."),
				variant: "destructive",
			});
		}
	};

	const onDeleteKey = async (keyId: string) => {
		try {
			await deleteKeyMutation.mutateAsync({
				params: {
					path: {
						keyId,
					},
				},
			});
			await queryClient.invalidateQueries({ queryKey: keysQueryKey });
			toast({ title: "Key deleted" });
		} catch (error) {
			toast({
				title: "Failed to delete key",
				description: getOpenApiErrorMessage(error, "Please try again."),
				variant: "destructive",
			});
		}
	};

	return (
		<div className="space-y-4">
			<div className="flex items-center justify-between">
				<h4 className="text-base font-semibold">Provider Keys</h4>
				<Dialog
					open={openCreate}
					onOpenChange={(nextOpen) => {
						setOpenCreate(nextOpen);
						if (!nextOpen) {
							createForm.reset({ token: "", label: "", status: "active" });
						}
					}}
				>
					<DialogTrigger asChild>
						<Button size="sm" variant="outline">
							<Plus className="mr-2 h-4 w-4" />
							Add Key
						</Button>
					</DialogTrigger>
					<DialogContent className="sm:max-w-[520px]">
						<DialogHeader>
							<DialogTitle>Add Provider Key</DialogTitle>
							<DialogDescription>
								Add a key for {provider.name}. The raw token is never shown
								again.
							</DialogDescription>
						</DialogHeader>
						<Form {...createForm}>
							<form
								onSubmit={createForm.handleSubmit(onCreateKey)}
								className="space-y-4"
							>
								<FormField
									control={createForm.control}
									name="label"
									render={({ field }) => (
										<FormItem>
											<FormLabel>Label (optional)</FormLabel>
											<FormControl>
												<Input placeholder="production" {...field} />
											</FormControl>
											<FormMessage />
										</FormItem>
									)}
								/>
								<FormField
									control={createForm.control}
									name="token"
									render={({ field }) => (
										<FormItem>
											<FormLabel>API Key</FormLabel>
											<FormControl>
												<Input
													type="password"
													placeholder="sk-..."
													{...field}
												/>
											</FormControl>
											<FormMessage />
										</FormItem>
									)}
								/>
								<FormField
									control={createForm.control}
									name="status"
									render={({ field }) => (
										<FormItem>
											<FormLabel>Status</FormLabel>
											<FormControl>
												<div className="flex items-center gap-2">
													<Button
														type="button"
														variant={
															field.value === "active" ? "default" : "outline"
														}
														onClick={() => field.onChange("active")}
													>
														Active
													</Button>
													<Button
														type="button"
														variant={
															field.value === "inactive" ? "default" : "outline"
														}
														onClick={() => field.onChange("inactive")}
													>
														Inactive
													</Button>
												</div>
											</FormControl>
											<FormMessage />
										</FormItem>
									)}
								/>
								<DialogFooter>
									<Button
										type="button"
										variant="outline"
										onClick={() => setOpenCreate(false)}
									>
										Cancel
									</Button>
									<Button type="submit" disabled={createKeyMutation.isPending}>
										{createKeyMutation.isPending ? (
											<>
												<Loader2 className="mr-2 h-4 w-4 animate-spin" />
												Saving
											</>
										) : (
											"Save"
										)}
									</Button>
								</DialogFooter>
							</form>
						</Form>
					</DialogContent>
				</Dialog>
			</div>

			{providerKeysQuery.isLoading ? (
				<div className="flex items-center gap-2 text-sm text-muted-foreground">
					<Loader2 className="h-4 w-4 animate-spin" />
					Loading keys
				</div>
			) : providerKeysQuery.isError ? (
				<div className="flex items-center gap-2 rounded-md border border-destructive/30 bg-destructive/5 p-3 text-sm text-destructive">
					<AlertTriangle className="h-4 w-4" />
					Unable to load provider keys.
				</div>
			) : keys.length === 0 ? (
				<div className="rounded-md border border-dashed p-4 text-sm text-muted-foreground">
					No keys added yet.
				</div>
			) : (
				<Table>
					<TableHeader>
						<TableRow>
							<TableHead>Label</TableHead>
							<TableHead>Masked token</TableHead>
							<TableHead>Status</TableHead>
							<TableHead>Created</TableHead>
							<TableHead className="text-right">Actions</TableHead>
						</TableRow>
					</TableHeader>
					<TableBody>
						{keys.map((key) => {
							const isEditing = editingKeyId === key.id;

							return (
								<TableRow key={key.id}>
									<TableCell>{key.label || "-"}</TableCell>
									<TableCell className="font-mono text-xs">
										{key.maskedToken}
									</TableCell>
									<TableCell>
										<StatusBadge status={key.status} variant="detailed" />
									</TableCell>
									<TableCell>{formatTimestamp(key.createdAt)}</TableCell>
									<TableCell className="text-right">
										<div className="flex items-center justify-end gap-2">
											<Button
												variant="outline"
												size="sm"
												onClick={() =>
													copyText(key.maskedToken, "Copied masked token")
												}
											>
												<Copy className="h-4 w-4" />
												<span className="sr-only">Copy masked token</span>
											</Button>

											<Dialog
												open={isEditing}
												onOpenChange={(nextOpen) => {
													setEditingKeyId(nextOpen ? key.id : null);
													if (nextOpen) {
														editForm.reset({
															label: key.label || "",
															status:
																key.status === "inactive"
																	? "inactive"
																	: "active",
														});
													}
												}}
											>
												<DialogTrigger asChild>
													<Button variant="outline" size="sm">
														<Pencil className="h-4 w-4" />
														<span className="sr-only">Edit key</span>
													</Button>
												</DialogTrigger>
												<DialogContent className="sm:max-w-[520px]">
													<DialogHeader>
														<DialogTitle>Edit Provider Key</DialogTitle>
														<DialogDescription>
															Update key label or status.
														</DialogDescription>
													</DialogHeader>
													<Form {...editForm}>
														<form
															onSubmit={editForm.handleSubmit((values) =>
																onEditKey(key.id, values),
															)}
															className="space-y-4"
														>
															<FormField
																control={editForm.control}
																name="label"
																render={({ field }) => (
																	<FormItem>
																		<FormLabel>Label</FormLabel>
																		<FormControl>
																			<Input
																				placeholder="production"
																				{...field}
																			/>
																		</FormControl>
																		<FormMessage />
																	</FormItem>
																)}
															/>
															<FormField
																control={editForm.control}
																name="status"
																render={({ field }) => (
																	<FormItem>
																		<FormLabel>Status</FormLabel>
																		<FormControl>
																			<div className="flex items-center gap-2">
																				<Button
																					type="button"
																					variant={
																						field.value === "active"
																							? "default"
																							: "outline"
																					}
																					onClick={() =>
																						field.onChange("active")
																					}
																				>
																					Active
																				</Button>
																				<Button
																					type="button"
																					variant={
																						field.value === "inactive"
																							? "default"
																							: "outline"
																					}
																					onClick={() =>
																						field.onChange("inactive")
																					}
																				>
																					Inactive
																				</Button>
																			</div>
																		</FormControl>
																		<FormMessage />
																	</FormItem>
																)}
															/>
															<DialogFooter>
																<Button
																	type="button"
																	variant="outline"
																	onClick={() => setEditingKeyId(null)}
																>
																	Cancel
																</Button>
																<Button
																	type="submit"
																	disabled={updateKeyMutation.isPending}
																>
																	{updateKeyMutation.isPending ? (
																		<>
																			<Loader2 className="mr-2 h-4 w-4 animate-spin" />
																			Saving
																		</>
																	) : (
																		"Save"
																	)}
																</Button>
															</DialogFooter>
														</form>
													</Form>
												</DialogContent>
											</Dialog>

											<Button
												variant="outline"
												size="sm"
												onClick={() => onDeleteKey(key.id)}
												disabled={deleteKeyMutation.isPending}
											>
												{deleteKeyMutation.isPending ? (
													<Loader2 className="h-4 w-4 animate-spin" />
												) : (
													<Trash className="h-4 w-4" />
												)}
												<span className="sr-only">Delete key</span>
											</Button>
										</div>
									</TableCell>
								</TableRow>
							);
						})}
					</TableBody>
				</Table>
			)}
		</div>
	);
}

interface AliasesSectionProps {
	provider: OpenAICompatibleProvider;
}

function AliasesSection({ provider }: AliasesSectionProps) {
	const api = useApi();
	const queryClient = useQueryClient();
	const [openCreate, setOpenCreate] = useState(false);
	const [editingAliasId, setEditingAliasId] = useState<string | null>(null);

	const createForm = useForm<CreateAliasFormValues>({
		resolver: zodResolver(aliasCreateSchema),
		defaultValues: {
			alias: "",
			modelId: "",
			status: "active",
		},
	});

	const editForm = useForm<UpdateAliasFormValues>({
		resolver: zodResolver(aliasUpdateSchema),
		defaultValues: {
			alias: "",
			modelId: "",
			status: "active",
		},
	});

	const aliasesQuery = api.useQuery(
		"get",
		"/openai-compatible-providers/{id}/aliases",
		{
			params: {
				path: {
					id: provider.id,
				},
			},
		},
	);

	const createAliasMutation = api.useMutation(
		"post",
		"/openai-compatible-providers/{id}/aliases",
	);
	const updateAliasMutation = api.useMutation(
		"patch",
		"/openai-compatible-providers/aliases/{aliasId}",
	);
	const deleteAliasMutation = api.useMutation(
		"delete",
		"/openai-compatible-providers/aliases/{aliasId}",
	);

	const aliases = aliasesQuery.data?.aliases ?? [];

	const aliasesQueryKey = api.queryOptions(
		"get",
		"/openai-compatible-providers/{id}/aliases",
		{
			params: {
				path: {
					id: provider.id,
				},
			},
		},
	).queryKey;

	const onCreateAlias = async (values: CreateAliasFormValues) => {
		try {
			await createAliasMutation.mutateAsync({
				params: {
					path: {
						id: provider.id,
					},
				},
				body: {
					alias: values.alias,
					modelId: values.modelId,
					status: values.status,
				},
			});
			await queryClient.invalidateQueries({ queryKey: aliasesQueryKey });
			toast({ title: "Alias added" });
			setOpenCreate(false);
			createForm.reset({ alias: "", modelId: "", status: "active" });
		} catch (error) {
			toast({
				title: "Failed to add alias",
				description: getOpenApiErrorMessage(error, "Please try again."),
				variant: "destructive",
			});
		}
	};

	const onEditAlias = async (
		aliasId: string,
		values: UpdateAliasFormValues,
	) => {
		try {
			await updateAliasMutation.mutateAsync({
				params: {
					path: {
						aliasId,
					},
				},
				body: {
					alias: values.alias,
					modelId: values.modelId,
					status: values.status,
				},
			});
			await queryClient.invalidateQueries({ queryKey: aliasesQueryKey });
			toast({ title: "Alias updated" });
			setEditingAliasId(null);
		} catch (error) {
			toast({
				title: "Failed to update alias",
				description: getOpenApiErrorMessage(error, "Please try again."),
				variant: "destructive",
			});
		}
	};

	const onDeleteAlias = async (aliasId: string) => {
		try {
			await deleteAliasMutation.mutateAsync({
				params: {
					path: {
						aliasId,
					},
				},
			});
			await queryClient.invalidateQueries({ queryKey: aliasesQueryKey });
			toast({ title: "Alias deleted" });
		} catch (error) {
			toast({
				title: "Failed to delete alias",
				description: getOpenApiErrorMessage(error, "Please try again."),
				variant: "destructive",
			});
		}
	};

	return (
		<div className="space-y-4">
			<div className="flex items-center justify-between">
				<h4 className="text-base font-semibold">Aliases</h4>
				<Dialog
					open={openCreate}
					onOpenChange={(nextOpen) => {
						setOpenCreate(nextOpen);
						if (!nextOpen) {
							createForm.reset({ alias: "", modelId: "", status: "active" });
						}
					}}
				>
					<DialogTrigger asChild>
						<Button size="sm" variant="outline">
							<Plus className="mr-2 h-4 w-4" />
							Add Alias
						</Button>
					</DialogTrigger>
					<DialogContent className="sm:max-w-[520px]">
						<DialogHeader>
							<DialogTitle>Add Alias</DialogTitle>
							<DialogDescription>
								Map a short alias to a model ID.
							</DialogDescription>
						</DialogHeader>
						<Form {...createForm}>
							<form
								onSubmit={createForm.handleSubmit(onCreateAlias)}
								className="space-y-4"
							>
								<FormField
									control={createForm.control}
									name="alias"
									render={({ field }) => (
										<FormItem>
											<FormLabel>Alias</FormLabel>
											<FormControl>
												<Input placeholder="gpt4o" {...field} />
											</FormControl>
											<FormMessage />
										</FormItem>
									)}
								/>
								<FormField
									control={createForm.control}
									name="modelId"
									render={({ field }) => (
										<FormItem>
											<FormLabel>Model ID</FormLabel>
											<FormControl>
												<Input placeholder="gpt-4.1" {...field} />
											</FormControl>
											<FormMessage />
										</FormItem>
									)}
								/>
								<FormField
									control={createForm.control}
									name="status"
									render={({ field }) => (
										<FormItem>
											<FormLabel>Status</FormLabel>
											<FormControl>
												<div className="flex items-center gap-2">
													<Button
														type="button"
														variant={
															field.value === "active" ? "default" : "outline"
														}
														onClick={() => field.onChange("active")}
													>
														Active
													</Button>
													<Button
														type="button"
														variant={
															field.value === "inactive" ? "default" : "outline"
														}
														onClick={() => field.onChange("inactive")}
													>
														Inactive
													</Button>
												</div>
											</FormControl>
											<FormMessage />
										</FormItem>
									)}
								/>
								<DialogFooter>
									<Button
										type="button"
										variant="outline"
										onClick={() => setOpenCreate(false)}
									>
										Cancel
									</Button>
									<Button
										type="submit"
										disabled={createAliasMutation.isPending}
									>
										{createAliasMutation.isPending ? (
											<>
												<Loader2 className="mr-2 h-4 w-4 animate-spin" />
												Saving
											</>
										) : (
											"Save"
										)}
									</Button>
								</DialogFooter>
							</form>
						</Form>
					</DialogContent>
				</Dialog>
			</div>

			{aliasesQuery.isLoading ? (
				<div className="flex items-center gap-2 text-sm text-muted-foreground">
					<Loader2 className="h-4 w-4 animate-spin" />
					Loading aliases
				</div>
			) : aliasesQuery.isError ? (
				<div className="flex items-center gap-2 rounded-md border border-destructive/30 bg-destructive/5 p-3 text-sm text-destructive">
					<AlertTriangle className="h-4 w-4" />
					Unable to load aliases.
				</div>
			) : aliases.length === 0 ? (
				<div className="rounded-md border border-dashed p-4 text-sm text-muted-foreground">
					No aliases added yet.
				</div>
			) : (
				<Table>
					<TableHeader>
						<TableRow>
							<TableHead>Alias</TableHead>
							<TableHead>Model ID</TableHead>
							<TableHead>Status</TableHead>
							<TableHead>Created</TableHead>
							<TableHead className="text-right">Actions</TableHead>
						</TableRow>
					</TableHeader>
					<TableBody>
						{aliases.map((alias) => {
							const isEditing = editingAliasId === alias.id;

							return (
								<TableRow key={alias.id}>
									<TableCell className="font-mono text-xs">
										{alias.alias}
									</TableCell>
									<TableCell className="font-mono text-xs">
										{alias.modelId}
									</TableCell>
									<TableCell>
										<StatusBadge status={alias.status} variant="detailed" />
									</TableCell>
									<TableCell>{formatTimestamp(alias.createdAt)}</TableCell>
									<TableCell className="text-right">
										<div className="flex items-center justify-end gap-2">
											<Button
												variant="outline"
												size="sm"
												onClick={() => copyText(alias.alias, "Copied alias")}
											>
												<Copy className="h-4 w-4" />
												<span className="sr-only">Copy alias</span>
											</Button>

											<Dialog
												open={isEditing}
												onOpenChange={(nextOpen) => {
													setEditingAliasId(nextOpen ? alias.id : null);
													if (nextOpen) {
														editForm.reset({
															alias: alias.alias,
															modelId: alias.modelId,
															status:
																alias.status === "inactive"
																	? "inactive"
																	: "active",
														});
													}
												}}
											>
												<DialogTrigger asChild>
													<Button variant="outline" size="sm">
														<Pencil className="h-4 w-4" />
														<span className="sr-only">Edit alias</span>
													</Button>
												</DialogTrigger>
												<DialogContent className="sm:max-w-[520px]">
													<DialogHeader>
														<DialogTitle>Edit Alias</DialogTitle>
														<DialogDescription>
															Update alias mapping and status.
														</DialogDescription>
													</DialogHeader>
													<Form {...editForm}>
														<form
															onSubmit={editForm.handleSubmit((values) =>
																onEditAlias(alias.id, values),
															)}
															className="space-y-4"
														>
															<FormField
																control={editForm.control}
																name="alias"
																render={({ field }) => (
																	<FormItem>
																		<FormLabel>Alias</FormLabel>
																		<FormControl>
																			<Input placeholder="gpt4o" {...field} />
																		</FormControl>
																		<FormMessage />
																	</FormItem>
																)}
															/>
															<FormField
																control={editForm.control}
																name="modelId"
																render={({ field }) => (
																	<FormItem>
																		<FormLabel>Model ID</FormLabel>
																		<FormControl>
																			<Input placeholder="gpt-4.1" {...field} />
																		</FormControl>
																		<FormMessage />
																	</FormItem>
																)}
															/>
															<FormField
																control={editForm.control}
																name="status"
																render={({ field }) => (
																	<FormItem>
																		<FormLabel>Status</FormLabel>
																		<FormControl>
																			<div className="flex items-center gap-2">
																				<Button
																					type="button"
																					variant={
																						field.value === "active"
																							? "default"
																							: "outline"
																					}
																					onClick={() =>
																						field.onChange("active")
																					}
																				>
																					Active
																				</Button>
																				<Button
																					type="button"
																					variant={
																						field.value === "inactive"
																							? "default"
																							: "outline"
																					}
																					onClick={() =>
																						field.onChange("inactive")
																					}
																				>
																					Inactive
																				</Button>
																			</div>
																		</FormControl>
																		<FormMessage />
																	</FormItem>
																)}
															/>
															<DialogFooter>
																<Button
																	type="button"
																	variant="outline"
																	onClick={() => setEditingAliasId(null)}
																>
																	Cancel
																</Button>
																<Button
																	type="submit"
																	disabled={updateAliasMutation.isPending}
																>
																	{updateAliasMutation.isPending ? (
																		<>
																			<Loader2 className="mr-2 h-4 w-4 animate-spin" />
																			Saving
																		</>
																	) : (
																		"Save"
																	)}
																</Button>
															</DialogFooter>
														</form>
													</Form>
												</DialogContent>
											</Dialog>

											<Button
												variant="outline"
												size="sm"
												onClick={() => onDeleteAlias(alias.id)}
												disabled={deleteAliasMutation.isPending}
											>
												{deleteAliasMutation.isPending ? (
													<Loader2 className="h-4 w-4 animate-spin" />
												) : (
													<Trash className="h-4 w-4" />
												)}
												<span className="sr-only">Delete alias</span>
											</Button>
										</div>
									</TableCell>
								</TableRow>
							);
						})}
					</TableBody>
				</Table>
			)}
		</div>
	);
}

interface ModelsSectionProps {
	provider: OpenAICompatibleProvider;
}

function ModelsSection({ provider }: ModelsSectionProps) {
	const api = useApi();
	const [search, setSearch] = useState("");
	const debouncedSearch = useDebouncedValue(search, 300);

	const modelsQuery = api.useQuery(
		"get",
		"/openai-compatible-providers/{id}/models",
		{
			params: {
				path: {
					id: provider.id,
				},
				query: {
					search: debouncedSearch || undefined,
				},
			},
		},
		{
			retry: false,
		},
	);

	const models = modelsQuery.data?.models ?? [];

	const refreshModels = () => {
		void modelsQuery.refetch();
	};

	return (
		<div className="space-y-4">
			<div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
				<h4 className="text-base font-semibold">Provider Models</h4>
				<div className="flex items-center gap-2">
					<div className="relative min-w-[280px]">
						<Search className="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
						<Input
							value={search}
							onChange={(event) => {
								setSearch(event.target.value);
							}}
							placeholder="Search models"
							className="pl-8"
						/>
					</div>
					<Button
						variant="outline"
						size="sm"
						onClick={refreshModels}
						disabled={modelsQuery.isFetching}
					>
						{modelsQuery.isFetching ? (
							<Loader2 className="h-4 w-4 animate-spin" />
						) : (
							<RefreshCw className="h-4 w-4" />
						)}
						<span className="sr-only">Refresh models</span>
					</Button>
				</div>
			</div>

			{modelsQuery.isLoading ? (
				<div className="flex items-center gap-2 text-sm text-muted-foreground">
					<Loader2 className="h-4 w-4 animate-spin" />
					Loading models
				</div>
			) : modelsQuery.isError ? (
				<div className="flex items-center gap-2 rounded-md border border-destructive/30 bg-destructive/5 p-3 text-sm text-destructive">
					<AlertTriangle className="h-4 w-4" />
					Unable to fetch models. Ensure this provider has an active key.
				</div>
			) : models.length === 0 ? (
				<div className="rounded-md border border-dashed p-4 text-sm text-muted-foreground">
					No models returned.
				</div>
			) : (
				<Table>
					<TableHeader>
						<TableRow>
							<TableHead>Model ID</TableHead>
							<TableHead className="text-right">Actions</TableHead>
						</TableRow>
					</TableHeader>
					<TableBody>
						{models.map((model: OpenAICompatibleModel) => (
							<TableRow key={model.id}>
								<TableCell className="font-mono text-xs">{model.id}</TableCell>
								<TableCell className="text-right">
									<Button
										variant="outline"
										size="sm"
										onClick={() => copyText(model.id, "Copied model ID")}
									>
										<Copy className="h-4 w-4" />
										<span className="sr-only">Copy model ID</span>
									</Button>
								</TableCell>
							</TableRow>
						))}
					</TableBody>
				</Table>
			)}
		</div>
	);
}

interface ProviderDetailsPaneProps {
	provider: OpenAICompatibleProvider;
	onProviderUpdated: (provider: OpenAICompatibleProvider) => void;
	onProviderDeleted: (providerId: string) => void;
}

function ProviderDetailsPane({
	provider,
	onProviderUpdated,
	onProviderDeleted,
}: ProviderDetailsPaneProps) {
	return (
		<Card>
			<CardHeader className="space-y-4">
				<div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
					<div className="space-y-1">
						<CardTitle className="text-xl">{provider.name}</CardTitle>
						<p className="text-sm text-muted-foreground">{provider.baseUrl}</p>
						<div className="flex items-center gap-2">
							<StatusBadge status={provider.status} variant="detailed" />
							<span className="text-xs text-muted-foreground">
								Created {formatTimestamp(provider.createdAt)}
							</span>
						</div>
					</div>
					<div className="flex items-center gap-2">
						<ProviderUpdateDialog
							provider={provider}
							onUpdated={onProviderUpdated}
						/>
						<ProviderDeleteButton
							provider={provider}
							onDeleted={onProviderDeleted}
						/>
					</div>
				</div>
			</CardHeader>
			<CardContent className="space-y-6">
				<Tabs defaultValue="keys" className="w-full">
					<TabsList>
						<TabsTrigger value="keys">Keys</TabsTrigger>
						<TabsTrigger value="aliases">Aliases</TabsTrigger>
						<TabsTrigger value="models">Models</TabsTrigger>
					</TabsList>
					<TabsContent value="keys" className="pt-2">
						<ProviderKeysSection provider={provider} />
					</TabsContent>
					<TabsContent value="aliases" className="pt-2">
						<AliasesSection provider={provider} />
					</TabsContent>
					<TabsContent value="models" className="pt-2">
						<ModelsSection provider={provider} />
					</TabsContent>
				</Tabs>
			</CardContent>
		</Card>
	);
}

export function OpenAICompatibleProvidersClient({
	initialProvidersData,
}: OpenAICompatibleProvidersClientProps) {
	const api = useApi();
	const { selectedOrganization } = useDashboardNavigation();
	const [selectedProviderId, setSelectedProviderId] = useState<string | null>(
		null,
	);
	const [searchInput, setSearchInput] = useState("");
	const search = useDebouncedValue(searchInput, 300);

	const providersQuery = api.useQuery(
		"get",
		"/openai-compatible-providers",
		{
			params: {
				query: {
					organizationId: selectedOrganization?.id,
					search: search || undefined,
				},
			},
		},
		{
			enabled: !!selectedOrganization?.id,
			...(initialProvidersData && !search
				? {
						initialData: initialProvidersData,
					}
				: {}),
		},
	);

	const providers = useMemo(() => {
		return providersQuery.data?.providers ?? [];
	}, [providersQuery.data?.providers]);

	const selectedProvider = useMemo(() => {
		if (!providers.length) {
			return undefined;
		}

		if (selectedProviderId) {
			return providers.find((provider) => provider.id === selectedProviderId);
		}

		return providers[0];
	}, [providers, selectedProviderId]);

	const selectedProviderName = selectedProvider?.name;

	if (!selectedOrganization) {
		return (
			<div className="flex flex-col">
				<div className="flex-1 space-y-4 p-4 pt-6 md:p-8">
					<div className="space-y-1">
						<h2 className="text-3xl font-bold tracking-tight">
							OpenAI-Compatible Providers
						</h2>
						<p className="text-muted-foreground">
							Select an organization to manage OpenAI-compatible providers.
						</p>
					</div>
				</div>
			</div>
		);
	}

	return (
		<div className="flex flex-col">
			<div className="flex-1 space-y-4 p-4 pt-6 md:p-8">
				<div className="flex flex-col gap-4 md:flex-row md:items-center md:justify-between">
					<div>
						<h2 className="text-3xl font-bold tracking-tight">
							OpenAI-Compatible Providers
						</h2>
						<p className="text-muted-foreground">
							Configure provider base URLs, attach multiple keys, and manage
							model aliases per provider.
						</p>
					</div>
					<ProviderCreateDialog
						organizationId={selectedOrganization.id}
						onCreated={(providerId) => {
							setSelectedProviderId(providerId);
						}}
					/>
				</div>

				<div className="grid gap-4 lg:grid-cols-[320px_1fr]">
					<Card className="h-fit">
						<CardHeader className="space-y-3">
							<CardTitle className="text-base">Providers</CardTitle>
							<div className="relative">
								<Search className="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
								<Input
									value={searchInput}
									onChange={(event) => {
										setSearchInput(event.target.value);
									}}
									placeholder="Search providers"
									className="pl-8"
								/>
							</div>
						</CardHeader>
						<CardContent>
							{providersQuery.isLoading ? (
								<div className="flex items-center gap-2 text-sm text-muted-foreground">
									<Loader2 className="h-4 w-4 animate-spin" />
									Loading providers
								</div>
							) : providersQuery.isError ? (
								<div className="flex items-center gap-2 rounded-md border border-destructive/30 bg-destructive/5 p-3 text-sm text-destructive">
									<AlertTriangle className="h-4 w-4" />
									Unable to load providers.
								</div>
							) : providers.length === 0 ? (
								<div className="rounded-md border border-dashed p-4 text-sm text-muted-foreground">
									No providers found.
								</div>
							) : (
								<div className="space-y-2">
									{providers.map((provider) => {
										const isSelected = selectedProvider?.id === provider.id;

										return (
											<button
												key={provider.id}
												type="button"
												onClick={() => setSelectedProviderId(provider.id)}
												className={`w-full rounded-md border px-3 py-2 text-left transition-colors ${
													isSelected
														? "border-primary bg-primary/5"
														: "border-border hover:bg-accent"
												}`}
											>
												<div className="flex items-center justify-between gap-2">
													<div className="min-w-0">
														<p className="truncate text-sm font-medium">
															{provider.name}
														</p>
														<p className="truncate text-xs text-muted-foreground">
															{provider.baseUrl}
														</p>
													</div>
													{isSelected ? (
														<Check className="h-4 w-4 text-primary" />
													) : null}
												</div>
												<div className="mt-2">
													<StatusBadge
														status={provider.status}
														variant="simple"
													/>
												</div>
											</button>
										);
									})}
								</div>
							)}
						</CardContent>
					</Card>

					{selectedProvider ? (
						<ProviderDetailsPane
							key={selectedProvider.id}
							provider={selectedProvider}
							onProviderUpdated={(updatedProvider) => {
								if (updatedProvider.id === selectedProvider.id) {
									setSelectedProviderId(updatedProvider.id);
								}
							}}
							onProviderDeleted={(deletedProviderId) => {
								if (deletedProviderId === selectedProvider.id) {
									setSelectedProviderId(null);
								}
							}}
						/>
					) : (
						<Card>
							<CardContent className="flex min-h-[320px] items-center justify-center p-6 text-sm text-muted-foreground">
								Select a provider to manage keys, aliases, and model discovery.
							</CardContent>
						</Card>
					)}
				</div>

				{selectedProviderName ? (
					<p className="text-xs text-muted-foreground">
						Active provider:{" "}
						<span className="font-medium">{selectedProviderName}</span>
					</p>
				) : null}
			</div>
		</div>
	);
}
