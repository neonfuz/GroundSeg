
class Setup:
    def handle_anchor(data, config, wg, urbit, minio):
        # set endpoint
        if 'skip' in data:
            config.config['firstBoot'] = False
            config.save_config()
            return 200

        changed = wg.change_url(data['endpoint'], urbit, minio)

        # register key
        if changed == 200:
            endpoint = config.config['endpointUrl']
            api_version = config.config['apiVersion']
            url = f"https://{endpoint}/{api_version}"
            if wg.build_anchor(url, data['key']):
                minio.start_mc()
                config.config['wgRegistered'] = True
                config.config['wgOn'] = True

                for patp in config.config['piers']:
                    urbit.register_urbit(patp, url)

                config.config['firstBoot'] = False
                if config.save_config():
                    return 200

        return 400