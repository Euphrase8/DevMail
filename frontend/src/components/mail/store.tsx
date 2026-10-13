import { createContext, useContext, useState, type ReactNode } from 'react';
import { initialData, type MailRecord } from '@/lib/mail/data';
type Store = { data: Record<string,MailRecord[]>; save:(section:string,item:MailRecord)=>void; remove:(section:string,id:string)=>void; notice:string; notify:(message:string)=>void; settings:Record<string,string>; setSetting:(key:string,value:string)=>void };
const Context = createContext<Store|null>(null);
export function MailProvider({children}:{children:ReactNode}) {
 const [data,setData]=useState(initialData); const [notice,notify]=useState(''); const [settings,setSettings]=useState<Record<string,string>>({name:'Acme workspace',timezone:'UTC',engine:'Mailcow',retention:'90',notifications:'On',mfa:'Off',api:'http://localhost:8081/api/v1'});
 const save=(section:string,item:MailRecord)=>setData(old=>({...old,[section]:(old[section]||[]).some(r=>r.id===item.id)?(old[section] || []).map(r=>r.id===item.id?item:r):[item,...(old[section]||[])]}));
 return <Context.Provider value={{data,save,remove:(section,id)=>setData(old=>({...old,[section]:(old[section] || []).filter(r=>r.id!==id)})),notice,notify,settings,setSetting:(key,value)=>setSettings(old=>({...old,[key]:value}))}}>{children}</Context.Provider>;
}
export function useMail(){const store=useContext(Context); if(!store)throw new Error('MailProvider is required'); return store;}
