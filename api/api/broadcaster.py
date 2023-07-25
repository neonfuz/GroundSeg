import json

from api.upload_broadcast import UploadBroadcast
from api.startram_broadcast import StarTramBroadcast
from api.urbits_broadcast import UrbitsBroadcast

class Broadcaster:
    def __init__(self,cfg,groundseg):
        self.cfg = cfg
        self.app = groundseg

    async def broadcast(self):
        a_broadcast = {
                "type": "structure",
                "auth_level": "authorized",
                "upload": UploadBroadcast(self.app).display(),
                "system": {
                    "usage": {
                        "ram": self.cfg._ram,
                        "cpu": self.cfg._cpu,
                        "cpu_temp": self.cfg._core_temp,
                        "disk": self.cfg._disk,
                        "swap": self.cfg.swap.active_swap(self.cfg.system.get('swapFile'))
                        },
                    "updates": {
                        "linux":self.cfg.linux_update_info
                        }
                    },
                "profile":{
                    "startram": StarTramBroadcast(self.app).display()
                    },
                "urbits": UrbitsBroadcast(self.app).display()
                }
        u_broadcast = {
                "type": "structure",
                "auth_level": "unauthorized",
                "login": {
                    "remainder": 0,
                    }
                }
        await self.authed(a_broadcast)
        await self.unauth(u_broadcast)

    async def setup(self):
        broadcast = {
                "type": "structure",
                "auth_level": "setup",
                "stage": self.app.setup.stage,
                "page": self.app.setup.page,
                "regions":self.app.wireguard.region_data
               }
        await self.authed(broadcast)
        await self.unauth(broadcast)

    async def authed(self, broadcast):
        try:
            sesh = self.app.active_sessions
            a = sesh.get('authorized').copy().keys()
            for s in a:
                try:
                    await s.send(json.dumps(broadcast))
                except:
                    print(f"removing {s}")
                    self.app.active_sessions['authorized'].pop(s)
        except Exception as e:
            print(f"api:broadcaster:authed: {e}")

    async def unauth(self, broadcast):
        try:
            sesh = self.app.active_sessions
            u = sesh.get('unauthorized').copy().keys()
            for s in u:
                try:
                    await s.send(json.dumps(broadcast))
                except:
                    print(f"removing {s}")
                    self.app.active_sessions['unauthorized'].pop(s)
        except Exception as e:
            print(f"api:broadcaster:unauth: {e}")
