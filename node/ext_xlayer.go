// Copyright 2025 The xlayer-geth Authors
// This file is part of xlayer-geth.
//
// xlayer-geth is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// xlayer-geth is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with xlayer-geth. If not, see <http://www.gnu.org/licenses/>.

package node

// Lifecycles exposes the lifecycles for xlayer-geth
// This allows xlayer-geth to access registered services like the Ethereum backend
func (n *Node) Lifecycles() []Lifecycle {
	n.lock.Lock()
	defer n.lock.Unlock()
	return n.lifecycles
}
