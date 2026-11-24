import { z } from "zod";
const base = import.meta.env.VITE_API_URL ?? "http://localhost:8080/api/v1";
let csrfToken = "";
const GraphNode=z.object({id:z.string(),type:z.string(),displayValue:z.string()});
const GraphEdge=z.object({id:z.string(),sourceEntityId:z.string(),destinationEntityId:z.string(),relationshipType:z.string(),confidence:z.number(),status:z.string(),assertionKind:z.string()});
const Schedule=z.object({id:z.string(),targetId:z.string(),collectors:z.array(z.string()),intervalHours:z.number(),enabled:z.boolean(),nextExecutionAt:z.string(),lastExecutedAt:z.string().nullable().optional()});
async function request<T>(path:string, init:RequestInit, schema:z.ZodType<T>):Promise<T>{const headers:Record<string,string>={"Content-Type":"application/json",...(init.headers as Record<string,string>??{})};if(init.method&&init.method!=="GET"&&csrfToken)headers["X-CSRF-Token"]=csrfToken;const response=await fetch(`${base}${path}`,{credentials:"include",...init,headers});const data=await response.json();if(!response.ok)throw new Error(data.error?.message??"Request failed");if(data.csrfToken){csrfToken=data.csrfToken;if(typeof sessionStorage!=="undefined")sessionStorage.setItem("atlas_csrf",csrfToken)}return schema.parse(data)}
export const commandApi={
 login:(username:string,password:string)=>request("/auth/login",{method:"POST",body:JSON.stringify({username,password})},z.object({user:z.object({id:z.string(),username:z.string()}),csrfToken:z.string()})),
 me:()=>request("/auth/me",{},z.object({user:z.object({id:z.string(),username:z.string()}),csrfToken:z.string()})),
 logout:()=>request("/auth/logout",{method:"POST"},z.object({status:z.string()})),
 dashboard:(id:string)=>request(`/targets/${id}/dashboard`,{},z.object({entityCount:z.number(),inScopeEntityCount:z.number(),outOfScopeObservationCount:z.number(),relationshipCount:z.number(),pendingNotes:z.number(),reviewedEntities:z.number(),entityCountsByType:z.record(z.string(),z.number()),recentRuns:z.array(z.any()),recentActivity:z.array(z.any())})),
 graph:(id:string,rootEntityId:string,depth=1)=>request(`/targets/${id}/graph?rootEntityId=${encodeURIComponent(rootEntityId)}&depth=${depth}&limit=100`,{},z.object({nodes:z.array(GraphNode),edges:z.array(GraphEdge),truncated:z.boolean()})),
 activity:(id:string)=>request(`/targets/${id}/activity`,{},z.object({data:z.array(z.object({id:z.string(),eventType:z.string(),message:z.string(),createdAt:z.string()}))})),
 notifications:()=>request("/notifications",{},z.object({data:z.array(z.object({id:z.string(),title:z.string(),message:z.string(),link:z.string(),readAt:z.string().nullable().optional(),createdAt:z.string()}))})),
 readNotification:(id:string)=>request(`/notifications/${id}`,{method:"PATCH",body:JSON.stringify({read:true})},z.object({status:z.string()})),
 schedules:(id:string)=>request(`/targets/${id}/schedules`,{},z.object({data:z.array(Schedule)})),
 createSchedule:(id:string,input:unknown)=>request(`/targets/${id}/schedules`,{method:"POST",body:JSON.stringify(input)},Schedule),
 setSchedule:(target:string,id:string,enabled:boolean)=>request(`/targets/${target}/schedules/${id}`,{method:"PATCH",body:JSON.stringify({enabled})},z.object({enabled:z.boolean()})),
 settings:()=>request("/settings",{},z.record(z.string(),z.unknown())),
 updateSettings:(input:unknown)=>request("/settings",{method:"PATCH",body:JSON.stringify(input)},z.record(z.string(),z.unknown())),
 exportURL:(target:string,format:"json"|"csv"|"markdown"|"graph")=>`${base}/targets/${target}/exports/${format}`,
};
