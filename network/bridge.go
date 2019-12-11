package network

type BridgeNetworkDriver struct {

}

func (d *BridgeNetworkDriver) Name() string  {
	return "bridge"
}

func (d *BridgeNetworkDriver) Create(subnet, name string) (*Network, error)  {

	return nil, nil
}

func (d *BridgeNetworkDriver) Delete(network Network) error  {

	return nil
}