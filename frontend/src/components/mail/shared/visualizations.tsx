import { useState } from 'react';
import { ArrowUpRight, Globe, Inbox, ShieldCheck, Server, Waypoints } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Link } from '@tanstack/react-router';
export function TrafficChart({period='24 hours'}:{period?:string}){
 const [hover,setHover]=useState<number|null>(null);
 const shift=period==='7 days'?12:period==='30 days'?23:0;
 const vals=Array.from({length:60},(_,i)=>55+Math.sin((i+shift)*.45)*19+Math.cos(i*.9)*12+(i>29?30:0)+(i>42&&i<48?35:0));
 const points=vals.map((v,i)=>`${i*10},${180-v}`).join(' ');
 const second=vals.map((v,i)=>`${i*10},${194-v*.62}`).join(' ');
 return <div className="relative mt-6" onMouseLeave={()=>setHover(null)}>
 <svg viewBox="0 0 600 220" className="w-full overflow-visible" role="img" aria-label={`Mail traffic over ${period}: delivered and received messages`} onMouseMove={e=>{const rect=e.currentTarget.getBoundingClientRect();setHover(Math.max(0,Math.min(59,Math.round((e.clientX-rect.left)/rect.width*59))));}}>
 {[40,90,140,190].map((y,i)=><g key={y}><line x1="0" x2="590" y1={y} y2={y} className="chart-grid"/><text x="0" y={y-7} className="chart-label">{['1.5k','1.0k','500','0'][i]}</text></g>)}
 <polygon points={`0,190 ${points} 590,190`} className="chart-area"/>
 <polyline points={second} className="chart-secondary"/><polyline points={points} className="chart-line"/>
 {['00:00','04:00','08:00','12:00','16:00','20:00','23:59'].map((v,i)=><text key={v} x={i*97} y="215" textAnchor={i===0?'start':i===6?'end':'middle'} className="chart-label">{period==='24 hours'?v:period==='7 days'?['Mon','Tue','Wed','Thu','Fri','Sat','Sun'][i]:`Oct ${1+i*4}`}</text>)}
 {hover!==null&&<g><line x1={hover*10} x2={hover*10} y1="10" y2="190" className="chart-cursor"/><circle cx={hover*10} cy={180-(vals[hover] ?? 0)} r="4" className="orbit-particle"/></g>}
 </svg>
 {hover!==null&&<div className="absolute top-0 right-0 border border-border bg-popover px-3 py-2 text-xs"><span className="text-muted-foreground">Delivered </span><span className="text-primary">{Math.round((vals[hover] ?? 0)*11)}</span></div>}
 </div>;
}
export function InfrastructureMap(){
 const [focus,setFocus]=useState('Mail flow');
 return <div className="topology relative min-h-[267px] overflow-hidden">
 <div className="absolute left-4 top-4 eyebrow">{focus}</div>
 <svg viewBox="0 0 450 270" className="h-[267px] w-full" role="img" aria-label="Animated topology connecting mail servers, domains, DNS, security and mailboxes">
 <ellipse cx="225" cy="139" rx="151" ry="59" transform="rotate(-15 225 139)" className="orbit-line"/>
 <ellipse cx="225" cy="139" rx="112" ry="42" transform="rotate(-15 225 139)" className="orbit-line" strokeDasharray="2 5"/>
 <path id="flow-orbit" d="M 79 178 A 151 59 -15 1 1 371 100 A 151 59 -15 1 1 79 178" className="orbit-path"/>
 {[0,3,6].map(delay=><circle r="2.5" key={delay} className="orbit-particle"><animateMotion dur="9s" begin={`${delay}s`} repeatCount="indefinite"><mpath href="#flow-orbit"/></animateMotion></circle>)}
 <line x1="225" y1="130" x2="225" y2="60" className="orbit-line"/>
 <line x1="225" y1="139" x2="360" y2="104" className="orbit-line"/>
 <line x1="225" y1="139" x2="323" y2="202" className="orbit-line"/>
 <line x1="225" y1="139" x2="133" y2="218" className="orbit-line"/>
 <line x1="225" y1="139" x2="74" y2="133" className="orbit-line"/>
 <circle cx="225" cy="139" r="27" className="orbit-center"/>
 <path d="M211 133 L225 125 L239 133 L225 141 Z M211 139 L225 147 L239 139 M211 145 L225 153 L239 145" className="chart-line"/>
 </svg>
 {[
 {name:'Domains',icon:Globe,pos:'left-[43%] top-[12%]',path:'/domains'},
 {name:'DNS',icon:Waypoints,pos:'right-[11%] top-[30%]',path:'/dns'},
 {name:'Security',icon:ShieldCheck,pos:'right-[16%] bottom-[10%]',path:'/monitoring'},
 {name:'Mailboxes',icon:Inbox,pos:'left-[22%] bottom-[5%]',path:'/mailboxes'},
 {name:'Servers',icon:Server,pos:'left-[8%] top-[43%]',path:'/servers'},
 ].map(n=><div className={`absolute ${n.pos} flex flex-col items-center gap-1.5`} key={n.name} onMouseEnter={()=>setFocus(n.name)} onMouseLeave={()=>setFocus('Mail flow')}><Button asChild variant="outline" size="icon" className="h-7 w-7 bg-background" aria-label={`View ${n.name}`}><Link to={n.path}><n.icon className="size-3.5"/></Link></Button><span className="font-mono text-[9px] text-muted-foreground">{n.name}</span></div>)}
 </div>;
}
export function Status({value}:{value:string}) {const kind=['Healthy','Active','Verified','Success','Completed'].includes(value)?'healthy':['Degraded','Warning','Missing'].includes(value)?'warning':['Failed','Suspended'].includes(value)?'error':'pending';return <span className={`status status-${kind}`}>{value}</span>;}
export function ViewLink({to,label='View all'}:{to:string;label?:string}){return <Button variant="ghost" size="sm" asChild className="h-7 px-0 text-[11px] text-muted-foreground"><Link to={to}>{label}<ArrowUpRight className="size-3"/></Link></Button>;}
